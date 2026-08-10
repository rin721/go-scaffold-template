package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/afero"
	"github.com/xuri/excelize/v2"
)

// impl 是 Storage 接口的具体实现
type impl struct {
	config  *Config
	mu      sync.RWMutex
	fs      afero.Fs
	watcher *fsnotify.Watcher
	watches map[string]*watchEntry // 路径 -> 监听条目
	closed  bool
}

// watchEntry 监听条目
type watchEntry struct {
	path    string
	handler WatchHandler
	cancel  context.CancelFunc
}

type workbook struct {
	file *excelize.File
}

func (w *workbook) SetCellValue(sheet string, cell string, value any) error {
	return w.file.SetCellValue(sheet, cell, value)
}

func (w *workbook) Close() error {
	return w.file.Close()
}

// New 创建新的 Storage 实例
func New(cfg *Config) (Storage, error) {
	if cfg == nil {
		cfg = &Config{}
		cfg.DefaultConfig()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	i := &impl{
		config:  cfg,
		watches: make(map[string]*watchEntry),
	}

	// 初始化文件系统
	if err := i.initFileSystem(); err != nil {
		return nil, err
	}

	// 初始化文件监听器
	if cfg.Local.EnableWatch {
		if err := i.initWatcher(); err != nil {
			return nil, err
		}
	}

	return i, nil
}

// initFileSystem 初始化文件系统
func (i *impl) initFileSystem() error {
	localCfg := i.config.normalizedLocal()
	switch localCfg.FSType {
	case FSTypeOS:
		i.fs = afero.NewOsFs()
	case FSTypeMemory:
		i.fs = afero.NewMemMapFs()
	case FSTypeReadOnly:
		i.fs = afero.NewReadOnlyFs(afero.NewOsFs())
	case FSTypeBasePathFS:
		i.fs = afero.NewBasePathFs(afero.NewOsFs(), localCfg.BasePath)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidFSType, localCfg.FSType)
	}
	return nil
}

// initWatcher 初始化文件监听器
func (i *impl) initWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("Storage: failed to create watcher: %w", err)
	}
	i.watcher = watcher
	return nil
}

// ReadFile 读取文件内容
func (i *impl) ReadFile(path string) ([]byte, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return afero.ReadFile(i.fs, path)
}

// WriteFile 写入文件内容
func (i *impl) WriteFile(path string, data []byte, perm os.FileMode) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return afero.WriteFile(i.fs, path, data, perm)
}

// Remove 删除文件或空目录
func (i *impl) Remove(path string) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.fs.Remove(path)
}

// RemoveAll 递归删除目录
func (i *impl) RemoveAll(path string) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.fs.RemoveAll(path)
}

// Exists 检查路径是否存在
func (i *impl) Exists(path string) (bool, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return afero.Exists(i.fs, path)
}

// MkdirAll 递归创建目录
func (i *impl) MkdirAll(path string, perm os.FileMode) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.fs.MkdirAll(path, perm)
}

// IsDir 判断是否为目录
func (i *impl) IsDir(path string) (bool, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return afero.IsDir(i.fs, path)
}

// IsFile 判断是否为文件
func (i *impl) IsFile(path string) (bool, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	isDir, err := afero.IsDir(i.fs, path)
	if err != nil {
		return false, err
	}
	return !isDir, nil
}

// FileSize 获取文件大小
func (i *impl) FileSize(path string) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	info, err := i.fs.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ListDir 列出目录内容
func (i *impl) ListDir(path string) ([]os.FileInfo, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return afero.ReadDir(i.fs, path)
}

// OpenWorkbook 打开 Excel 工作簿。
func (i *impl) OpenWorkbook(path string) (Workbook, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// 读取文件内容
	data, err := afero.ReadFile(i.fs, path)
	if err != nil {
		return nil, fmt.Errorf("Storage: failed to read excel file: %w", err)
	}

	// 从字节流打开
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Storage: failed to parse excel file: %w", err)
	}

	return &workbook{file: file}, nil
}

// CreateWorkbook 创建新的 Excel 工作簿。
func (i *impl) CreateWorkbook() Workbook {
	return &workbook{file: excelize.NewFile()}
}

// SaveWorkbook 保存 Excel 工作簿。
func (i *impl) SaveWorkbook(book Workbook, path string) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if book == nil {
		return fmt.Errorf("%w: workbook is nil", ErrInvalidConfig)
	}
	internal, ok := book.(*workbook)
	if !ok {
		return fmt.Errorf("%w: unsupported workbook implementation %T", ErrInvalidConfig, book)
	}

	// 保存到缓冲区
	buf, err := internal.file.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("Storage: failed to write excel to buffer: %w", err)
	}

	// 写入文件系统
	if err := afero.WriteFile(i.fs, path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("Storage: failed to save excel file: %w", err)
	}

	return nil
}

// ReadExcelSheet 读取 Excel 工作表数据
func (i *impl) ReadExcelSheet(path, sheet string) ([][]string, error) {
	book, err := i.OpenWorkbook(path)
	if err != nil {
		return nil, err
	}
	defer book.Close()

	internal, ok := book.(*workbook)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported workbook implementation %T", ErrInvalidConfig, book)
	}

	rows, err := internal.file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("Storage: failed to read sheet %s: %w", sheet, err)
	}

	return rows, nil
}

// OpenImage 打开图片文件
func (i *impl) OpenImage(path string) (image.Image, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// 读取文件内容
	data, err := afero.ReadFile(i.fs, path)
	if err != nil {
		return nil, fmt.Errorf("Storage: failed to read image file: %w", err)
	}

	// 解码图片
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Storage: failed to decode image: %w", err)
	}

	return img, nil
}

// SaveImage 保存图片文件
func (i *impl) SaveImage(img image.Image, path string, format ImageFormat) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	underlyingFormat, err := toImagingFormat(format)
	if err != nil {
		return err
	}

	// 编码图片到缓冲区
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, underlyingFormat); err != nil {
		return fmt.Errorf("Storage: failed to encode image: %w", err)
	}

	// 写入文件系统
	if err := afero.WriteFile(i.fs, path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("Storage: failed to save image file: %w", err)
	}

	return nil
}

// ResizeImage 调整图片大小
func (i *impl) ResizeImage(src, dst string, width, height int, format ImageFormat) error {
	// 打开图片
	img, err := i.OpenImage(src)
	if err != nil {
		return err
	}

	// 调整大小
	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	// 保存图片
	return i.SaveImage(resized, dst, format)
}

// CropImage 裁剪图片
func (i *impl) CropImage(src, dst string, rect image.Rectangle, format ImageFormat) error {
	// 打开图片
	img, err := i.OpenImage(src)
	if err != nil {
		return err
	}

	// 裁剪
	cropped := imaging.Crop(img, rect)

	// 保存图片
	return i.SaveImage(cropped, dst, format)
}

func toImagingFormat(format ImageFormat) (imaging.Format, error) {
	switch format {
	case ImageFormatJPEG:
		return imaging.JPEG, nil
	case ImageFormatPNG:
		return imaging.PNG, nil
	case ImageFormatGIF:
		return imaging.GIF, nil
	case ImageFormatTIFF:
		return imaging.TIFF, nil
	case ImageFormatBMP:
		return imaging.BMP, nil
	default:
		return imaging.PNG, fmt.Errorf("%w: unsupported image format %q", ErrInvalidConfig, format)
	}
}

// Close 关闭文件服务
func (i *impl) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.closed {
		return nil
	}

	// 停止所有监听
	if i.watcher != nil {
		for path := range i.watches {
			i.watcher.Remove(path)
		}
		i.watcher.Close()
	}

	i.closed = true
	return nil
}

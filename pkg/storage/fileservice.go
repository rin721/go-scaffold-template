package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

import (
	"image"
	"os"
	"time"
)

// Storage 定义文件服务的接口
// 提供统一的文件操作API,集成多个开源库提供强大的文件处理能力
//
// 设计目标:
//   - 抽象文件系统操作,支持多种文件系统类型
//   - 集成文件监听、复制、MIME检测等功能
//   - 支持 Excel 和图片文件的专业处理
//   - 提供统一、易用的接口
//
// 使用示例:
//
//	cfg := &Storage.Config{
//	    FSType: Storage.FSTypeOS,
//	    BasePath: "./data",
//	}
//	fs, err := Storage.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer fs.Close()
//
//	// 读写文件
//	err = fs.WriteFile("test.txt", []byte("hello"), 0644)
//	data, err := fs.ReadFile("test.txt")
type Storage interface {
	// ===== 基础文件系统操作 =====

	// ReadFile 读取文件内容
	// 参数:
	//   path: 文件路径
	// 返回:
	//   []byte: 文件内容
	//   error: 读取失败时的错误
	ReadFile(path string) ([]byte, error)

	// WriteFile 写入文件内容
	// 参数:
	//   path: 文件路径
	//   data: 要写入的数据
	//   perm: 文件权限
	// 返回:
	//   error: 写入失败时的错误
	WriteFile(path string, data []byte, perm os.FileMode) error

	// Remove 删除文件或空目录
	// 参数:
	//   path: 路径
	// 返回:
	//   error: 删除失败时的错误
	Remove(path string) error

	// RemoveAll 递归删除目录及其内容
	// 参数:
	//   path: 目录路径
	// 返回:
	//   error: 删除失败时的错误
	RemoveAll(path string) error

	// Exists 检查路径是否存在
	// 参数:
	//   path: 路径
	// 返回:
	//   bool: 是否存在
	//   error: 检查失败时的错误
	Exists(path string) (bool, error)

	// MkdirAll 递归创建目录
	// 参数:
	//   path: 目录路径
	//   perm: 目录权限
	// 返回:
	//   error: 创建失败时的错误
	MkdirAll(path string, perm os.FileMode) error

	// IsDir 判断路径是否为目录
	// 参数:
	//   path: 路径
	// 返回:
	//   bool: 是否为目录
	//   error: 检查失败时的错误
	IsDir(path string) (bool, error)

	// IsFile 判断路径是否为文件
	// 参数:
	//   path: 路径
	// 返回:
	//   bool: 是否为文件
	//   error: 检查失败时的错误
	IsFile(path string) (bool, error)

	// FileSize 获取文件大小
	// 参数:
	//   path: 文件路径
	// 返回:
	//   int64: 文件大小(字节)
	//   error: 获取失败时的错误
	FileSize(path string) (int64, error)

	// ListDir 列出目录内容
	// 参数:
	//   path: 目录路径
	// 返回:
	//   []os.FileInfo: 文件信息列表
	//   error: 列出失败时的错误
	ListDir(path string) ([]os.FileInfo, error)

	// ===== 文件复制功能 (基于 otiai10/copy) =====

	// Copy 复制单个文件
	// 参数:
	//   src: 源文件路径
	//   dst: 目标文件路径
	//   opts: 复制选项
	// 返回:
	//   error: 复制失败时的错误
	Copy(src, dst string, opts ...CopyOption) error

	// CopyDir 递归复制目录
	// 参数:
	//   src: 源目录路径
	//   dst: 目标目录路径
	//   opts: 复制选项
	// 返回:
	//   error: 复制失败时的错误
	CopyDir(src, dst string, opts ...CopyOption) error

	// ===== MIME类型检测 (基于 mimetype) =====

	// DetectMIME 从文件路径检测MIME类型
	// 参数:
	//   path: 文件路径
	// 返回:
	//   string: MIME类型 (如 "image/jpeg")
	//   error: 检测失败时的错误
	DetectMIME(path string) (string, error)

	// DetectMIMEFromBytes 从字节数据检测MIME类型
	// 参数:
	//   data: 文件数据
	// 返回:
	//   string: MIME类型
	//   error: 检测失败时的错误
	DetectMIMEFromBytes(data []byte) (string, error)

	// ===== 文件监听功能 (基于 fsnotify) =====

	// Watch 监听文件或目录的变化
	// 参数:
	//   path: 要监听的路径
	//   handler: 事件处理函数
	// 返回:
	//   error: 监听失败时的错误
	// 注意:
	//   - 如果路径已被监听,返回 ErrWatcherAlreadyExists
	//   - handler 在独立的 goroutine 中执行
	Watch(path string, handler WatchHandler) error

	// StopWatch 停止监听指定路径
	// 参数:
	//   path: 路径
	// 返回:
	//   error: 停止失败时的错误(如路径未被监听)
	StopWatch(path string) error

	// StopAllWatch 停止所有监听
	StopAllWatch()

	// ===== Excel 文件处理 (基于 excelize) =====

	// OpenWorkbook 打开 Excel 工作簿。
	// 参数:
	//   path: Excel 文件路径
	// 返回:
	//   Workbook: 项目自有工作簿接口
	//   error: 打开失败时的错误
	OpenWorkbook(path string) (Workbook, error)

	// CreateWorkbook 创建新的 Excel 工作簿。
	// 返回:
	//   Workbook: 项目自有工作簿接口
	CreateWorkbook() Workbook

	// SaveWorkbook 保存 Excel 工作簿。
	// 参数:
	//   workbook: Excel 工作簿
	//   path: 保存路径
	// 返回:
	//   error: 保存失败时的错误
	SaveWorkbook(workbook Workbook, path string) error

	// ReadExcelSheet 读取 Excel 工作表数据
	// 参数:
	//   path: Excel 文件路径
	//   sheet: 工作表名称
	// 返回:
	//   [][]string: 二维字符串数组(行x列)
	//   error: 读取失败时的错误
	ReadExcelSheet(path, sheet string) ([][]string, error)

	// ===== 图片处理功能 (基于 imaging) =====

	// OpenImage 打开图片文件
	// 参数:
	//   path: 图片文件路径
	// 返回:
	//   image.Image: 图片对象
	//   error: 打开失败时的错误
	OpenImage(path string) (image.Image, error)

	// SaveImage 保存图片文件
	// 参数:
	//   img: 图片对象
	//   path: 保存路径
	//   format: 项目图片格式常量
	// 返回:
	//   error: 保存失败时的错误
	SaveImage(img image.Image, path string, format ImageFormat) error

	// ResizeImage 调整图片大小
	// 参数:
	//   src: 源图片路径
	//   dst: 目标图片路径
	//   width: 目标宽度 (0表示按比例)
	//   height: 目标高度 (0表示按比例)
	//   format: 输出格式
	// 返回:
	//   error: 处理失败时的错误
	ResizeImage(src, dst string, width, height int, format ImageFormat) error

	// CropImage 裁剪图片
	// 参数:
	//   src: 源图片路径
	//   dst: 目标图片路径
	//   rect: 裁剪区域
	//   format: 输出格式
	// 返回:
	//   error: 处理失败时的错误
	CropImage(src, dst string, rect image.Rectangle, format ImageFormat) error

	// ===== 生命周期管理 =====

	// Close 关闭文件服务,释放资源
	// 返回:
	//   error: 关闭失败时的错误
	Close() error
}

// WatchHandler 文件监听事件处理函数
type WatchHandler func(event WatchEvent)

// WatchEvent 文件监听事件
type WatchEvent struct {
	// Path 发生变化的文件路径
	Path string

	// Op 操作类型 (CREATE, WRITE, REMOVE, RENAME, CHMOD)
	Op string

	// Time 事件时间
	Time time.Time

	// IsDir 是否为目录
	IsDir bool

	// Error 保存监听后端返回的异步错误；仅在 Op 为 ERROR 时有值。
	Error error `json:"-"`
}

// Workbook 是业务代码操作 Excel 工作簿时依赖的项目自有接口。
type Workbook interface {
	SetCellValue(sheet string, cell string, value any) error
	Close() error
}

// ImageFormat 表示存储包支持的图片编码格式。
type ImageFormat string

const (
	// ImageFormatJPEG 表示 JPEG 图片。
	ImageFormatJPEG ImageFormat = "jpeg"
	// ImageFormatPNG 表示 PNG 图片。
	ImageFormatPNG ImageFormat = "png"
	// ImageFormatGIF 表示 GIF 图片。
	ImageFormatGIF ImageFormat = "gif"
	// ImageFormatTIFF 表示 TIFF 图片。
	ImageFormatTIFF ImageFormat = "tiff"
	// ImageFormatBMP 表示 BMP 图片。
	ImageFormatBMP ImageFormat = "bmp"
)

// CopyOption 文件复制选项接口
type CopyOption interface {
	apply(*copyOptions)
}

// copyOptions 复制选项
type copyOptions struct {
	// OnSymlink 符号链接处理策略
	OnSymlink func(src string) SymlinkAction

	// Skip 跳过函数
	Skip func(src string) bool

	// PreserveTimes 是否保留时间戳
	PreserveTimes bool

	// Sync 是否同步到磁盘
	Sync bool
}

// SymlinkAction 符号链接处理动作
type SymlinkAction int

const (
	// SymlinkShallow 浅复制符号链接
	SymlinkShallow SymlinkAction = iota

	// SymlinkDeep 深复制符号链接指向的内容
	SymlinkDeep

	// SymlinkSkip 跳过符号链接
	SymlinkSkip
)

// copyOptionFunc 选项函数适配器
type copyOptionFunc func(*copyOptions)

// apply 将文件服务选项写入服务实例，用于在默认行为基础上覆写局部能力。
func (f copyOptionFunc) apply(opts *copyOptions) {
	f(opts)
}

// WithPreserveTimes 设置保留时间戳
func WithPreserveTimes(preserve bool) CopyOption {
	return copyOptionFunc(func(opts *copyOptions) {
		opts.PreserveTimes = preserve
	})
}

// WithSync 设置同步到磁盘
func WithSync(sync bool) CopyOption {
	return copyOptionFunc(func(opts *copyOptions) {
		opts.Sync = sync
	})
}

// WithSkip 设置跳过函数
func WithSkip(skip func(string) bool) CopyOption {
	return copyOptionFunc(func(opts *copyOptions) {
		opts.Skip = skip
	})
}

package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/otiai10/copy"
	"github.com/spf13/afero"
)

// Copy 复制单个文件
func (i *impl) Copy(src, dst string, opts ...CopyOption) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// 应用选项
	options := &copyOptions{}
	for _, opt := range opts {
		opt.apply(options)
	}

	// 检查源文件是否存在
	exists, err := afero.Exists(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to check source file: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: %s", ErrPathNotFound, src)
	}

	// 检查是否为文件
	isDir, err := afero.IsDir(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to check source type: %w", err)
	}
	if isDir {
		return fmt.Errorf("%w: %s is a directory, use CopyDir instead", ErrNotFile, src)
	}

	// 读取源文件内容
	data, err := afero.ReadFile(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to read source file: %w", err)
	}

	// 获取源文件权限
	srcInfo, err := i.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("Storage: failed to get source file info: %w", err)
	}

	// 写入目标文件
	if err := afero.WriteFile(i.fs, dst, data, srcInfo.Mode()); err != nil {
		return fmt.Errorf("Storage: failed to write destination file: %w", err)
	}

	// 如果需要保留时间戳
	if options.PreserveTimes {
		if err := i.fs.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
			return fmt.Errorf("Storage: failed to preserve times: %w", err)
		}
	}

	return nil
}

// CopyDir 递归复制目录
func (i *impl) CopyDir(src, dst string, opts ...CopyOption) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// 应用选项
	options := &copyOptions{}
	for _, opt := range opts {
		opt.apply(options)
	}

	// 检查源目录是否存在
	exists, err := afero.Exists(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to check source directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: %s", ErrPathNotFound, src)
	}

	// 检查是否为目录
	isDir, err := afero.IsDir(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to check source type: %w", err)
	}
	if !isDir {
		return fmt.Errorf("%w: %s is a file, use Copy instead", ErrNotDirectory, src)
	}

	// 对于 OS 文件系统,使用 otiai10/copy 库获得更好的性能
	if i.config.normalizedLocal().FSType == FSTypeOS {
		return i.copyDirWithLib(src, dst, options)
	}

	// 对于其他文件系统,使用 afero 实现
	return i.copyDirWithAfero(src, dst, options)
}

// copyDirWithLib 使用 otiai10/copy 库复制目录
func (i *impl) copyDirWithLib(src, dst string, options *copyOptions) error {
	copyOpts := copy.Options{
		PreserveTimes: options.PreserveTimes,
		Sync:          options.Sync,
	}

	// 设置符号链接处理策略
	if options.OnSymlink != nil {
		copyOpts.OnSymlink = func(s string) copy.SymlinkAction {
			action := options.OnSymlink(s)
			switch action {
			case SymlinkShallow:
				return copy.Shallow
			case SymlinkDeep:
				return copy.Deep
			case SymlinkSkip:
				return copy.Skip
			default:
				return copy.Shallow
			}
		}
	}

	// 设置跳过函数
	if options.Skip != nil {
		copyOpts.Skip = func(_ os.FileInfo, src, _ string) (bool, error) {
			return options.Skip(src), nil
		}
	}

	// 执行复制
	if err := copy.Copy(src, dst, copyOpts); err != nil {
		return fmt.Errorf("Storage: failed to copy directory: %w", err)
	}

	return nil
}

// copyDirWithAfero 使用 afero 递归复制目录
func (i *impl) copyDirWithAfero(src, dst string, options *copyOptions) error {
	// 创建目标目录
	srcInfo, err := i.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("Storage: failed to get source directory info: %w", err)
	}

	if err := i.fs.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("Storage: failed to create destination directory: %w", err)
	}

	// 读取源目录内容
	entries, err := afero.ReadDir(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to read source directory: %w", err)
	}

	// 遍历并复制每个条目
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// 检查是否需要跳过
		if options.Skip != nil && options.Skip(srcPath) {
			continue
		}

		if entry.IsDir() {
			// 递归复制目录
			if err := i.copyDirWithAfero(srcPath, dstPath, options); err != nil {
				return err
			}
		} else {
			// 复制文件
			if err := i.copyFileInternal(srcPath, dstPath, options); err != nil {
				return err
			}
		}
	}

	// 保留时间戳
	if options.PreserveTimes {
		if err := i.fs.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
			return fmt.Errorf("Storage: failed to preserve directory times: %w", err)
		}
	}

	return nil
}

// copyFileInternal 内部文件复制方法
func (i *impl) copyFileInternal(src, dst string, options *copyOptions) error {
	// 读取源文件
	data, err := afero.ReadFile(i.fs, src)
	if err != nil {
		return fmt.Errorf("Storage: failed to read file: %w", err)
	}

	// 获取源文件信息
	srcInfo, err := i.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("Storage: failed to get file info: %w", err)
	}

	// 写入目标文件
	if err := afero.WriteFile(i.fs, dst, data, srcInfo.Mode()); err != nil {
		return fmt.Errorf("Storage: failed to write file: %w", err)
	}

	// 保留时间戳
	if options.PreserveTimes {
		if err := i.fs.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
			return fmt.Errorf("Storage: failed to preserve file times: %w", err)
		}
	}

	return nil
}

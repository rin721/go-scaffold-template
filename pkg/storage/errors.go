package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

import "errors"

var (
	// ErrInvalidConfig 无效配置错误
	ErrInvalidConfig = errors.New("Storage: invalid configuration")

	// ErrPathNotFound 路径不存在错误
	ErrPathNotFound = errors.New("Storage: path not found")

	// ErrNotDirectory 非目录错误
	ErrNotDirectory = errors.New("Storage: not a directory")

	// ErrNotFile 非文件错误
	ErrNotFile = errors.New("Storage: not a file")

	// ErrWatcherNotFound 监听器不存在错误
	ErrWatcherNotFound = errors.New("Storage: watcher not found")

	// ErrInvalidFSType 无效的文件系统类型
	ErrInvalidFSType = errors.New("Storage: invalid filesystem type")

	// ErrWatcherAlreadyExists 监听器已存在错误
	ErrWatcherAlreadyExists = errors.New("Storage: watcher already exists for this path")
)

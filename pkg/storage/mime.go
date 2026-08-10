package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

import (
	"fmt"

	"github.com/gabriel-vasile/mimetype"
	"github.com/spf13/afero"
)

// DetectMIME 从文件路径检测MIME类型
func (i *impl) DetectMIME(path string) (string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// 读取文件内容
	data, err := afero.ReadFile(i.fs, path)
	if err != nil {
		return "", fmt.Errorf("Storage: failed to read file for MIME detection: %w", err)
	}

	// 检测MIME类型
	mtype := mimetype.Detect(data)
	return mtype.String(), nil
}

// DetectMIMEFromBytes 从字节数据检测MIME类型
func (i *impl) DetectMIMEFromBytes(data []byte) (string, error) {
	mtype := mimetype.Detect(data)
	return mtype.String(), nil
}

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

// StreamingClient 是对象存储的流式扩展契约。
type StreamingClient interface {
	StorageClient
	PutStream(ctx context.Context, key string, reader io.Reader, opts PutOptions) error
	GetStream(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
}

// PresignClient 是对象存储的预签名 URL 扩展契约。
type PresignClient interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (string, error)
}

// ObjectPolicy 约束对象路径和 MIME。
type ObjectPolicy struct {
	AllowedMIMETypes map[string]struct{}
	MaxBytes         int64
}

// ValidateObject 校验对象大小和 MIME。
func (p ObjectPolicy) ValidateObject(info ObjectInfo) error {
	if p.MaxBytes > 0 && info.Size > p.MaxBytes {
		return fmt.Errorf("object %s exceeds %d bytes", info.Key, p.MaxBytes)
	}
	if len(p.AllowedMIMETypes) > 0 {
		if _, ok := p.AllowedMIMETypes[info.ContentType]; !ok {
			return fmt.Errorf("object %s content type %q is not allowed", info.Key, info.ContentType)
		}
	}
	return nil
}

// PutStreamFallback 为只支持字节接口的 client 提供流式写入适配。
func PutStreamFallback(ctx context.Context, client StorageClient, key string, reader io.Reader, opts PutOptions) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return client.Put(ctx, key, data, opts)
}

// GetStreamFallback 为只支持字节接口的 client 提供流式读取适配。
func GetStreamFallback(ctx context.Context, client StorageClient, key string) (io.ReadCloser, ObjectInfo, error) {
	data, info, err := client.Get(ctx, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return io.NopCloser(bytes.NewReader(data)), info, nil
}

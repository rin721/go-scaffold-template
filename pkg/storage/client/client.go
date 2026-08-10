package client

import "context"

// PutOptions describes optional object metadata used by object-style storage.
type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

// ObjectInfo is the portable metadata returned by StorageClient implementations.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
	Metadata    map[string]string
}

// StorageClient is the object-storage contract shared by local, R2, MinIO, and
// future storage backends. It is intentionally narrower than the legacy afero
// based Storage file toolbox so object stores do not need to emulate a full FS.
type StorageClient interface {
	Put(ctx context.Context, key string, data []byte, opts PutOptions) error
	Get(ctx context.Context, key string) ([]byte, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

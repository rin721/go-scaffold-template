package cache

import "errors"

var (
	ErrNotFound           = errors.New("cache value not found")
	ErrNilContext         = errors.New("cache context is nil")
	ErrEmptyKey           = errors.New("cache key is empty")
	ErrInvalidTTL         = errors.New("cache ttl is invalid")
	ErrNilRemoteStore     = errors.New("cache remote store is nil")
	ErrInvalidCachedValue = errors.New("cache value is invalid")
	// ErrClientClosed 表示 typed Client 已关闭，不能继续使用。
	ErrClientClosed = errors.New("cache client is closed")
	// ErrDisabled 表示共享缓存后端被明确禁用。
	ErrDisabled = errors.New("cache backend is disabled")
)

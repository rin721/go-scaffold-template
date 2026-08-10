package cache

import "errors"

var (
	ErrNotFound           = errors.New("cache value not found")
	ErrNilContext         = errors.New("cache context is nil")
	ErrEmptyKey           = errors.New("cache key is empty")
	ErrInvalidTTL         = errors.New("cache ttl is invalid")
	ErrNilRemoteStore     = errors.New("cache remote store is nil")
	ErrInvalidCachedValue = errors.New("cache value is invalid")
)

package core

import "errors"

var (
	ErrNilContext              = errors.New("database context is nil")
	ErrInvalidEngine           = errors.New("database engine is invalid")
	ErrInvalidDriver           = errors.New("database driver is invalid")
	ErrEmptyDSN                = errors.New("database dsn is empty")
	ErrInvalidPoolConfig       = errors.New("database pool config is invalid")
	ErrNilTransactionFunc      = errors.New("database transaction function is nil")
	ErrLastInsertIDUnsupported = errors.New("database last insert id is unsupported")
	ErrNotFound                = errors.New("database row not found")
)

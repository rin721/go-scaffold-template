package database

import "errors"

var (
	ErrNilContext          = errors.New("database context is nil")
	ErrNilConfig           = errors.New("database config is nil")
	ErrInvalidDriver       = errors.New("database driver is invalid")
	ErrEmptyDSN            = errors.New("database dsn is empty")
	ErrInvalidPoolConfig   = errors.New("database pool config is invalid")
	ErrNilClientFunc       = errors.New("database client function is nil")
	ErrNilTransactionFunc  = errors.New("database transaction function is nil")
	ErrClientUnavailable   = errors.New("database client is unavailable")
	ErrOperationFailed     = errors.New("database operation failed")
	ErrInvalidSchema       = errors.New("database schema is invalid")
	ErrInvalidQuery        = errors.New("database query is invalid")
	ErrUnsafeMutation      = errors.New("database mutation requires filters")
	ErrNotFound            = errors.New("database row not found")
	ErrDuplicateKey        = errors.New("database duplicate key")
	ErrForeignKeyViolation = errors.New("database foreign key violation")
	ErrOptimisticConflict  = errors.New("database optimistic lock conflict")
)

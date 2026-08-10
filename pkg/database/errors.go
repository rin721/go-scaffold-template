package database

import "github.com/rin721/go-scaffold2/pkg/database/internal/core"

var (
	ErrNilContext              = core.ErrNilContext
	ErrInvalidEngine           = core.ErrInvalidEngine
	ErrInvalidDriver           = core.ErrInvalidDriver
	ErrEmptyDSN                = core.ErrEmptyDSN
	ErrInvalidPoolConfig       = core.ErrInvalidPoolConfig
	ErrNilTransactionFunc      = core.ErrNilTransactionFunc
	ErrLastInsertIDUnsupported = core.ErrLastInsertIDUnsupported
	ErrNotFound                = core.ErrNotFound
)

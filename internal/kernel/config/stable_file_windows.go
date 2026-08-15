//go:build windows

package config

import (
	"errors"
	"os"
	"syscall"
)

func isTransientFileReadError(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, errFileChangedDuringRead) ||
		errors.Is(err, syscall.Errno(32)) ||
		errors.Is(err, syscall.Errno(33))
}

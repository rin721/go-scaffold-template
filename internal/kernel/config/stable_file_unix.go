//go:build !windows

package config

import (
	"errors"
	"os"
)

func isTransientFileReadError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, errFileChangedDuringRead)
}

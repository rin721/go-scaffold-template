//go:build !windows

package config

import "os"

func replaceDefaultFile(source, target string) error {
	return os.Rename(source, target)
}

package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	defaultDirectoryMode = 0o700
	defaultFileMode      = 0o600
)

type temporaryDefaultFile interface {
	Name() string
	Chmod(fs.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type defaultFileOperations interface {
	MkdirAll(string, fs.FileMode) error
	CreateTemp(string, string) (temporaryDefaultFile, error)
	Link(string, string) error
	Remove(string) error
	Replace(string, string) error
}

type osDefaultFileOperations struct{}

func (osDefaultFileOperations) MkdirAll(path string, mode fs.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (osDefaultFileOperations) CreateTemp(directory, pattern string) (temporaryDefaultFile, error) {
	return os.CreateTemp(directory, pattern)
}

func (osDefaultFileOperations) Link(source, target string) error { return os.Link(source, target) }
func (osDefaultFileOperations) Remove(path string) error         { return os.Remove(path) }
func (osDefaultFileOperations) Replace(source, target string) error {
	return replaceDefaultFile(source, target)
}

func writeDefaultFile(target string, payload []byte, force bool) error {
	return writeDefaultFileWithOperations(target, payload, force, osDefaultFileOperations{})
}

func writeDefaultFileWithOperations(target string, payload []byte, force bool, operations defaultFileOperations) (err error) {
	directory := filepath.Dir(target)
	if err := operations.MkdirAll(directory, defaultDirectoryMode); err != nil {
		return fmt.Errorf("create default configuration directory: %w", err)
	}
	temporary, err := operations.CreateTemp(directory, ".default-config-*")
	if err != nil {
		return fmt.Errorf("create temporary default configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if cleanupErr := operations.Remove(temporaryPath); cleanupErr != nil && !errors.Is(cleanupErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove temporary default configuration: %w", cleanupErr))
		}
	}()

	if chmodErr := temporary.Chmod(defaultFileMode); chmodErr != nil {
		return errors.Join(fmt.Errorf("set temporary default configuration permissions: %w", chmodErr), closeFile(temporary))
	}
	written, writeErr := temporary.Write(payload)
	if writeErr == nil && written != len(payload) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		return errors.Join(fmt.Errorf("write temporary default configuration: %w", writeErr), closeFile(temporary))
	}
	if syncErr := temporary.Sync(); syncErr != nil {
		return errors.Join(fmt.Errorf("sync temporary default configuration: %w", syncErr), closeFile(temporary))
	}
	if closeErr := temporary.Close(); closeErr != nil {
		return fmt.Errorf("close temporary default configuration: %w", closeErr)
	}

	if force {
		if replaceErr := operations.Replace(temporaryPath, target); replaceErr != nil {
			return fmt.Errorf("replace default configuration: %w", replaceErr)
		}
		return nil
	}
	if linkErr := operations.Link(temporaryPath, target); linkErr != nil {
		if errors.Is(linkErr, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrTargetExists, target)
		}
		return fmt.Errorf("create default configuration target: %w", linkErr)
	}
	return nil
}

func closeFile(file temporaryDefaultFile) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary default configuration: %w", err)
	}
	return nil
}

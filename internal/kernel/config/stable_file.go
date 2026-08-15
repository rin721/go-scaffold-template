package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	stableFileReadAttempts = 12
	stableFileReadInterval = 10 * time.Millisecond
)

type fileSample struct {
	data             []byte
	size             int64
	modifiedUnixNano int64
}

type stableFileReader struct {
	readFile func(string) ([]byte, error)
	stat     func(string) (os.FileInfo, error)
	attempts int
	interval time.Duration
}

func readStableFile(ctx context.Context, path string) ([]byte, error) {
	reader := stableFileReader{
		readFile: os.ReadFile,
		stat:     os.Stat,
		attempts: stableFileReadAttempts,
		interval: stableFileReadInterval,
	}
	return reader.read(ctx, path)
}

func (r stableFileReader) read(ctx context.Context, path string) ([]byte, error) {
	if ctx == nil {
		return nil, fmt.Errorf("stable file read context is nil")
	}
	if path == "" {
		return nil, fmt.Errorf("stable file read path is empty")
	}
	if r.readFile == nil || r.stat == nil {
		return nil, fmt.Errorf("stable file reader is incomplete")
	}
	if r.attempts < 2 {
		return nil, fmt.Errorf("stable file reader requires at least two attempts")
	}
	if r.interval <= 0 {
		return nil, fmt.Errorf("stable file reader interval must be positive")
	}

	var previous *fileSample
	var lastTransient error
	for attempt := 1; attempt <= r.attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current, err := r.sample(path)
		if err != nil {
			if !isTransientFileReadError(err) {
				return nil, err
			}
			lastTransient = err
			previous = nil
		} else if previous != nil && previous.equal(current) {
			return append([]byte(nil), current.data...), nil
		} else {
			previous = &current
			lastTransient = nil
		}
		if attempt == r.attempts {
			break
		}
		if err := waitStableFileRetry(ctx, r.interval); err != nil {
			return nil, err
		}
	}
	if lastTransient != nil {
		return nil, fmt.Errorf("read stable config file after %d attempts: %w", r.attempts, lastTransient)
	}
	return nil, fmt.Errorf("config file did not stabilize after %d attempts", r.attempts)
}

func (r stableFileReader) sample(path string) (fileSample, error) {
	data, err := r.readFile(path)
	if err != nil {
		return fileSample{}, err
	}
	info, err := r.stat(path)
	if err != nil {
		return fileSample{}, err
	}
	if !info.Mode().IsRegular() {
		return fileSample{}, fmt.Errorf("config file %s is not a regular file", path)
	}
	if info.Size() != int64(len(data)) {
		return fileSample{}, errFileChangedDuringRead
	}
	return fileSample{data: data, size: info.Size(), modifiedUnixNano: info.ModTime().UnixNano()}, nil
}

func (s fileSample) equal(other fileSample) bool {
	return s.size == other.size &&
		s.modifiedUnixNano == other.modifiedUnixNano &&
		bytes.Equal(s.data, other.data)
}

func waitStableFileRetry(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var errFileChangedDuringRead = errors.New("config file changed during read")

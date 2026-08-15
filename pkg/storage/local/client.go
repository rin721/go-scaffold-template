package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	storageclient "github.com/rin721/go-scaffold-template/pkg/storage/client"
)

// Config configures the local object-storage adapter.
type Config struct {
	BasePath string
}

type Client struct {
	basePath string
}

// New creates a local object-storage adapter rooted at BasePath.
func New(cfg Config) (*Client, error) {
	basePath := strings.TrimSpace(cfg.BasePath)
	if basePath == "" {
		basePath = "."
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage base path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("create local storage base path: %w", err)
	}
	return &Client{basePath: abs}, nil
}

func (c *Client) Put(_ context.Context, key string, data []byte, _ storageclient.PutOptions) error {
	path, err := c.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local object dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Client) Get(_ context.Context, key string) ([]byte, storageclient.ObjectInfo, error) {
	path, err := c.pathForKey(key)
	if err != nil {
		return nil, storageclient.ObjectInfo{}, err
	}
	// #nosec G304 -- pathForKey 已做绝对根边界和穿越校验，读取被限制在 basePath 内。
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, storageclient.ObjectInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, storageclient.ObjectInfo{}, err
	}
	return data, storageclient.ObjectInfo{Key: key, Size: info.Size()}, nil
}

func (c *Client) Delete(_ context.Context, key string) error {
	path, err := c.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (c *Client) Exists(_ context.Context, key string) (bool, error) {
	path, err := c.pathForKey(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (c *Client) HealthCheck(ctx context.Context) error {
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("create local storage healthcheck key: %w", err)
	}
	key := ".setup-healthcheck-" + hex.EncodeToString(suffix[:])
	if err := c.Put(ctx, key, []byte("ok"), storageclient.PutOptions{ContentType: "text/plain"}); err != nil {
		return err
	}
	data, _, err := c.Get(ctx, key)
	if err != nil {
		return errors.Join(err, c.Delete(context.Background(), key))
	}
	if string(data) != "ok" {
		return errors.Join(fmt.Errorf("unexpected local storage healthcheck payload"), c.Delete(context.Background(), key))
	}
	exists, err := c.Exists(ctx, key)
	if err != nil {
		return errors.Join(err, c.Delete(context.Background(), key))
	}
	if !exists {
		return errors.Join(fmt.Errorf("local storage healthcheck object is missing"), c.Delete(context.Background(), key))
	}
	return c.Delete(context.Background(), key)
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) pathForKey(key string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(key))
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("object key is required")
	}
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("object key escapes storage root: %s", key)
	}
	path := filepath.Join(c.basePath, cleaned)
	rel, err := filepath.Rel(c.basePath, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("object key escapes storage root: %s", key)
	}
	return path, nil
}

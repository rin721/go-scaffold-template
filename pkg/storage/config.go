package storage

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Driver Driver       `mapstructure:"driver"`
	Local  LocalConfig  `mapstructure:"local"`
	S3     ObjectConfig `mapstructure:"s3"`
	MinIO  ObjectConfig `mapstructure:"minio"`
}

func (c *Config) ValidateName() string {
	return "storage"
}

// Validate validates the local file-service portion used by New.
func (c *Config) Validate() error {
	local := c.normalizedLocal()
	switch local.FSType {
	case FSTypeOS, FSTypeMemory, FSTypeReadOnly, FSTypeBasePathFS:
	default:
		return fmt.Errorf("%w: %s", ErrInvalidFSType, local.FSType)
	}
	if local.FSType == FSTypeBasePathFS && local.BasePath == "" {
		return fmt.Errorf("%w: local.basePath is required for basepath filesystem", ErrInvalidConfig)
	}
	if local.WatchBufferSize < 0 {
		return fmt.Errorf("%w: local.watchBufferSize must be non-negative", ErrInvalidConfig)
	}
	return nil
}

func (c *Config) DefaultConfig() {
	c.Driver = DriverLocal
	c.Local = LocalConfig{
		FSType:          DefaultFSType,
		BasePath:        DefaultBasePath,
		EnableWatch:     true,
		WatchBufferSize: 100,
	}
}

func (c *Config) OverrideConfig() {
	if driver := os.Getenv("STORAGE_DRIVER"); driver != "" {
		c.Driver = Driver(strings.ToLower(strings.TrimSpace(driver)))
	}
	if basePath := os.Getenv("STORAGE_LOCAL_BASE_PATH"); basePath != "" {
		c.Local.BasePath = basePath
	}
	if publicURL := os.Getenv("STORAGE_LOCAL_PUBLIC_URL"); publicURL != "" {
		c.Local.PublicURL = publicURL
	}
	if fsType := os.Getenv("STORAGE_LOCAL_FS_TYPE"); fsType != "" {
		c.Local.FSType = FSType(fsType)
	}
	if enableWatch := os.Getenv("STORAGE_LOCAL_ENABLE_WATCH"); enableWatch != "" {
		if val, err := strconv.ParseBool(enableWatch); err == nil {
			c.Local.EnableWatch = val
		}
	}
	if bufferSize := os.Getenv("STORAGE_LOCAL_WATCH_BUFFER_SIZE"); bufferSize != "" {
		if val, err := strconv.Atoi(bufferSize); err == nil {
			c.Local.WatchBufferSize = val
		}
	}
	overrideObjectConfigFromEnv(&c.S3, "STORAGE_S3_")
	overrideObjectConfigFromEnv(&c.MinIO, "STORAGE_MINIO_")
}

func (c *Config) normalizedLocal() LocalConfig {
	local := c.Local
	if local.FSType == "" {
		local.FSType = DefaultFSType
	}
	if local.BasePath == "" {
		local.BasePath = DefaultBasePath
	}
	return local
}

func overrideObjectConfigFromEnv(cfg *ObjectConfig, prefix string) {
	if endpoint := os.Getenv(prefix + "ENDPOINT"); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if region := os.Getenv(prefix + "REGION"); region != "" {
		cfg.Region = region
	}
	if bucket := os.Getenv(prefix + "BUCKET"); bucket != "" {
		cfg.Bucket = bucket
	}
	if accessKey := os.Getenv(prefix + "ACCESS_KEY_ID"); accessKey != "" {
		cfg.AccessKeyID = accessKey
	}
	if secretKey := os.Getenv(prefix + "SECRET_ACCESS_KEY"); secretKey != "" {
		cfg.SecretAccessKey = secretKey
	}
	if pathStyle := os.Getenv(prefix + "USE_PATH_STYLE"); pathStyle != "" {
		if val, err := strconv.ParseBool(pathStyle); err == nil {
			cfg.PathStyle = val
		}
	}
	if publicBaseURL := os.Getenv(prefix + "PUBLIC_BASE_URL"); publicBaseURL != "" {
		cfg.PublicBaseURL = publicBaseURL
	}
}

package s3compat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	storageclient "github.com/rin721/go-scaffold-template/pkg/storage/client"
)

const (
	ProviderS3    = "s3"
	ProviderR2    = "r2"
	ProviderMinIO = "minio"
)

// Config configures an S3-compatible object storage backend.
type Config struct {
	Provider        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool
	PublicBaseURL   string
}

type Client struct {
	bucket string
	client *s3.Client
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load s3-compatible config: %w", err)
	}
	s3Client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.PathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &Client{bucket: cfg.Bucket, client: s3Client}, nil
}

func (c *Client) Put(ctx context.Context, key string, data []byte, opts storageclient.PutOptions) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: stringPtr(opts.ContentType),
		Metadata:    opts.Metadata,
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, storageclient.ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return nil, storageclient.ObjectInfo{}, err
	}
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, storageclient.ObjectInfo{}, fmt.Errorf("get object %s: %w", key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, storageclient.ObjectInfo{}, fmt.Errorf("read object %s: %w", key, err)
	}
	info := storageclient.ObjectInfo{
		Key:         key,
		ContentType: stringValue(out.ContentType),
		ETag:        stringValue(out.ETag),
		Metadata:    out.Metadata,
	}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	return data, info, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if err := validateKey(key); err != nil {
		return false, err
	}
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return false, nil
		}
	}
	return false, fmt.Errorf("head object %s: %w", key, err)
}

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	if err != nil {
		return fmt.Errorf("head bucket %s: %w", c.bucket, err)
	}
	return nil
}

func (c *Client) Close() error {
	return nil
}

func normalizeConfig(cfg Config) Config {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	if cfg.Region == "" {
		if cfg.Provider == ProviderR2 {
			cfg.Region = "auto"
		} else {
			cfg.Region = "us-east-1"
		}
	}
	if cfg.Provider == ProviderMinIO {
		cfg.PathStyle = true
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.Provider != ProviderS3 && cfg.Provider != ProviderR2 && cfg.Provider != ProviderMinIO {
		return fmt.Errorf("unsupported object storage provider %q", cfg.Provider)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("object storage endpoint is required")
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("object storage bucket is required")
	}
	if cfg.AccessKeyID == "" {
		return fmt.Errorf("object storage access key id is required")
	}
	if cfg.SecretAccessKey == "" {
		return fmt.Errorf("object storage secret access key is required")
	}
	return nil
}

func validateKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") || strings.Contains(key, "../") || key == ".." {
		return fmt.Errorf("invalid object key %q", key)
	}
	return nil
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return aws.String(value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

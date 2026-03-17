package objectstore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/FischukSergey/otus-ms/internal/config"
)

// S3Store реализует Store поверх S3-совместимого API.
type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store создаёт S3-хранилище на базе AWS SDK v2.
func NewS3Store(cfg config.ObjectStorageConfig) (*S3Store, error) {
	endpoint := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = &endpoint
	})

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// PutText сохраняет текстовый контент в объектное хранилище.
func (s *S3Store) PutText(ctx context.Context, key string, content string) (string, error) {
	if key == "" {
		return "", errors.New("put text: key is empty")
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        strings.NewReader(content),
		ContentType: ptr("text/plain; charset=utf-8"),
	})
	if err != nil {
		return "", fmt.Errorf("put object key=%s: %w", key, err)
	}

	return key, nil
}

func normalizeEndpoint(endpoint string, useSSL bool) string {
	trimmed := strings.TrimSpace(strings.TrimRight(endpoint, "/"))
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}

	scheme := "https://"
	if !useSSL {
		scheme = "http://"
	}
	return scheme + trimmed
}

func ptr(s string) *string {
	return &s
}

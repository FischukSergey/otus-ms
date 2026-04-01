package objectstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

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

// ListAndDeleteOld перебирает все объекты бакета с заданным prefix, определяет дату
// из пути ключа (формат prefix/yyyy/mm/dd/...) и удаляет те, чья дата старше olderThan.
// Удаление выполняется батчами по 1000 объектов (лимит S3 API).
// Возвращает количество удалённых объектов.
func (s *S3Store) ListAndDeleteOld(ctx context.Context, prefix string, olderThan time.Time) (int, error) {
	cutoffDate := olderThan.UTC().Truncate(24 * time.Hour)

	var toDelete []types.ObjectIdentifier
	var continuationToken *string

	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &s.bucket,
			Prefix:            &prefix,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return 0, fmt.Errorf("list objects prefix=%s: %w", prefix, err)
		}

		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			objDate, ok := parseDateFromKey(*obj.Key, prefix)
			if !ok {
				continue
			}
			if objDate.Before(cutoffDate) {
				key := strings.Clone(*obj.Key)
				toDelete = append(toDelete, types.ObjectIdentifier{Key: &key})
			}
		}

		if out.IsTruncated == nil || !*out.IsTruncated || out.NextContinuationToken == nil {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	deleted, err := s.deleteObjects(ctx, toDelete)
	if err != nil {
		return deleted, err
	}

	return deleted, nil
}

// deleteObjects удаляет объекты батчами по 1000 (лимит S3 DeleteObjects API).
func (s *S3Store) deleteObjects(ctx context.Context, objects []types.ObjectIdentifier) (int, error) {
	const batchSize = 1000
	var totalDeleted int

	for i := 0; i < len(objects); i += batchSize {
		batch := objects[i:min(i+batchSize, len(objects))]

		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &s.bucket,
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   ptr2(true),
			},
		})
		if err != nil {
			return totalDeleted, fmt.Errorf("delete objects batch: %w", err)
		}
		if len(out.Errors) > 0 {
			e := out.Errors[0]
			return totalDeleted, fmt.Errorf("delete object key=%s: %s", ptrStr(e.Key), ptrStr(e.Message))
		}

		totalDeleted += len(batch)
	}

	return totalDeleted, nil
}

// parseDateFromKey парсит дату из ключа S3 формата <prefix>/yyyy/mm/dd/<file>.
// Возвращает false если ключ не соответствует ожидаемому формату.
func parseDateFromKey(key, prefix string) (time.Time, bool) {
	trimmed := strings.TrimPrefix(key, strings.Trim(prefix, "/")+"/")
	// ожидаем yyyy/mm/dd/...
	parts := strings.SplitN(trimmed, "/", 4)
	if len(parts) < 3 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006/01/02", parts[0]+"/"+parts[1]+"/"+parts[2])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func ptr2(b bool) *bool { return &b }

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

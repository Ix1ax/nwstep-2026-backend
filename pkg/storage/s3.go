package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
)

type S3Storage struct {
	client *minio.Client
	log    zerolog.Logger
}

func NewS3Client(cfg *config.Config, log zerolog.Logger) (*S3Storage, error) {
	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKey, cfg.S3.SecretKey, ""),
		Secure: cfg.S3.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init s3 client: %w", err)
	}

	storage := &S3Storage{
		client: client,
		log:    log,
	}

	if err := storage.EnsureBucket(context.Background(), cfg.S3.Bucket); err != nil {
		return nil, err
	}

	return storage, nil
}

func (s *S3Storage) EnsureBucket(ctx context.Context, bucketName string) error {
	exists, err := s.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		s.log.Info().Str("bucket", bucketName).Msg("Created new S3 bucket")
	}
	return nil
}

func (s *S3Storage) Upload(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *S3Storage) GetPresignedURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (s *S3Storage) Delete(ctx context.Context, bucketName, objectName string) error {
	return s.client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
}

package upload

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/storage"
)

type UploadService struct {
	db *sqlx.DB
	s3 *storage.S3Storage
}

func NewUploadService(db *sqlx.DB, s3 *storage.S3Storage) *UploadService {
	return &UploadService{db: db, s3: s3}
}

func (s *UploadService) UploadFile(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*UploadResponse, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := filepath.Ext(fileHeader.Filename)
	id := uuid.New().String()
	s3Key := fmt.Sprintf("%s/%s%s", userID, id, ext)
	contentType := fileHeader.Header.Get("Content-Type")

	err = s.s3.Upload(ctx, "uploads", s3Key, file, fileHeader.Size, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to s3: %w", err)
	}

	metadata := &FileMetadata{
		ID:          id,
		UserID:      userID,
		Filename:    fileHeader.Filename,
		S3Key:       s3Key,
		ContentType: contentType,
		Size:        fileHeader.Size,
		CreatedAt:   time.Now(),
	}

	query := `
		INSERT INTO files (id, user_id, filename, s3_key, content_type, size, created_at)
		VALUES (:id, :user_id, :filename, :s3_key, :content_type, :size, :created_at)
	`
	_, err = s.db.NamedExecContext(ctx, query, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	url, err := s.s3.GetPresignedURL(ctx, "uploads", s3Key, 24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		ID:  id,
		URL: url,
	}, nil
}

func (s *UploadService) GetFile(ctx context.Context, id string) (*FileMetadata, string, error) {
	var metadata FileMetadata
	query := `SELECT * FROM files WHERE id = $1`
	err := s.db.GetContext(ctx, &metadata, query, id)
	if err != nil {
		return nil, "", err
	}

	url, err := s.s3.GetPresignedURL(ctx, "uploads", metadata.S3Key, 1*time.Hour)
	if err != nil {
		return nil, "", err
	}

	return &metadata, url, nil
}

func (s *UploadService) DeleteFile(ctx context.Context, id, userID string) error {
	var metadata FileMetadata
	query := `SELECT * FROM files WHERE id = $1`
	err := s.db.GetContext(ctx, &metadata, query, id)
	if err != nil {
		return err
	}

	if metadata.UserID != userID {
		return fmt.Errorf("unauthorized to delete this file")
	}

	err = s.s3.Delete(ctx, "uploads", metadata.S3Key)
	if err != nil {
		return err
	}

	deleteQuery := `DELETE FROM files WHERE id = $1`
	_, err = s.db.ExecContext(ctx, deleteQuery, id)
	return err
}

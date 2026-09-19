package upload

import "time"

type FileMetadata struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"user_id"`
	Filename    string    `db:"filename" json:"filename"`
	S3Key       string    `db:"s3_key" json:"s3_key"`
	ContentType string    `db:"content_type" json:"content_type"`
	Size        int64     `db:"size" json:"size"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

type UploadResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

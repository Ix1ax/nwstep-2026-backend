package upload

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/storage"
)

type Module struct {
	handler *UploadHandler
}

func NewModule(db *sqlx.DB, s3 *storage.S3Storage, log zerolog.Logger) *Module {
	service := NewUploadService(db, s3)
	handler := NewUploadHandler(service)
	return &Module{handler: handler}
}

func (m *Module) Register(router fiber.Router, authMiddleware fiber.Handler) {
	group := router.Group("/upload")
	
	group.Get("/:id", m.handler.GetFile)
	group.Get("/:id/view", m.handler.ViewFile)

	protected := group.Group("", authMiddleware)
	protected.Post("/", m.handler.Upload)
	protected.Delete("/:id", m.handler.DeleteFile)
}

package upload

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/auth"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type UploadHandler struct {
	service *UploadService
}

func NewUploadHandler(service *UploadService) *UploadHandler {
	return &UploadHandler{service: service}
}

// Upload godoc
// @Summary Upload file
// @Tags upload
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Success 201 {object} response.Response{data=UploadResponse}
// @Failure 400 {object} response.Response
// @Router /upload [post]
func (h *UploadHandler) Upload(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "missing file in request")
	}
	
	res, err := h.service.UploadFile(c.Context(), userID, file)
	if err != nil {
		return response.InternalError(c, err.Error())
	}
	
	return response.Created(c, res)
}

// GetFile godoc
// @Summary Get file details and URL
// @Tags upload
// @Produce json
// @Param id path string true "File ID"
// @Success 200 {object} response.Response{data=UploadResponse}
// @Router /upload/{id} [get]
func (h *UploadHandler) GetFile(c *fiber.Ctx) error {
	id := c.Params("id")

	_, url, err := h.service.GetFile(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "file not found")
	}

	return response.OK(c, fiber.Map{
		"id":  id,
		"url": url,
	})
}

// ViewFile godoc
// @Summary View / stream file directly (redirects to S3 URL)
// @Tags upload
// @Param id path string true "File ID"
// @Success 307
// @Router /upload/{id}/view [get]
func (h *UploadHandler) ViewFile(c *fiber.Ctx) error {
	id := c.Params("id")

	_, url, err := h.service.GetFile(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "file not found")
	}

	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// DeleteFile godoc
// @Summary Delete file
// @Tags upload
// @Produce json
// @Security BearerAuth
// @Param id path string true "File ID"
// @Success 200 {object} response.Response
// @Router /upload/{id} [delete]
func (h *UploadHandler) DeleteFile(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := auth.GetUserID(c)
	
	if err := h.service.DeleteFile(c.Context(), id, userID); err != nil {
		return response.BadRequest(c, err.Error())
	}
	
	return response.OK(c, fiber.Map{"deleted": true})
}

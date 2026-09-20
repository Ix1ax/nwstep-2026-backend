package choice

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Allocate calculates the energy transfer and system reaction from the slider.
// @Summary Allocate energy between colonies
// @Description Execute real-time physical resource transfer between non-biological entities
// @Tags choice
// @Accept json
// @Produce json
// @Param request body AllocationRequest true "Allocation parameters"
// @Success 200 {object} response.Response{data=AllocationResponse}
// @Router /choice/allocate [post]
func (h *Handler) Allocate(c *fiber.Ctx) error {
	var req AllocationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.service.ExecuteAllocation(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, result)
}

// ListDilemmas returns the available collective decision scenarios.
// @Summary List dilemmas
// @Description Get crisis scenarios and available alternative choices
// @Tags choice
// @Produce json
// @Success 200 {object} response.Response{data=[]Dilemma}
// @Router /choice/dilemmas [get]
func (h *Handler) ListDilemmas(c *fiber.Ctx) error {
	dilemmas := h.service.GetDilemmas()
	return response.OK(c, dilemmas)
}

// ResolveDilemma calculates the optimal collective choice using the specified philosophy.
// @Summary Resolve dilemma via Choice Machine
// @Description Apply Bentham, Rawls, Quadratic, or Entropy optimization to a crisis
// @Tags choice
// @Accept json
// @Produce json
// @Param request body ResolveDilemmaRequest true "Philosophy and dilemma ID"
// @Success 200 {object} response.Response{data=ResolveDilemmaResponse}
// @Router /choice/dilemmas/resolve [post]
func (h *Handler) ResolveDilemma(c *fiber.Ctx) error {
	var req ResolveDilemmaRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.service.ResolveDilemma(req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, result)
}

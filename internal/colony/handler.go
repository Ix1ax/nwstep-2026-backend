package colony

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

// ListColonies returns all active non-biological colonies.
// @Summary List all colonies
// @Description Retrieve current states, signals, and telemetry for all colonies
// @Tags colonies
// @Produce json
// @Success 200 {object} response.Response{data=[]Colony}
// @Router /colonies [get]
func (h *Handler) ListColonies(c *fiber.Ctx) error {
	colonies := h.service.GetColonies()
	return response.OK(c, colonies)
}

// GetColony returns a single colony by ID.
// @Summary Get colony by ID
// @Description Retrieve details of a specific colony
// @Tags colonies
// @Produce json
// @Param id path string true "Colony ID"
// @Success 200 {object} response.Response{data=Colony}
// @Failure 404 {object} response.Response
// @Router /colonies/{id} [get]
func (h *Handler) GetColony(c *fiber.Ctx) error {
	id := c.Params("id")
	col, err := h.service.GetColony(id)
	if err != nil {
		return response.NotFound(c, "Colony not found")
	}
	return response.OK(c, col)
}

// GetEnvironment returns the current state of the planetary sandbox.
// @Summary Get environment state
// @Description Retrieve ambient temperature, solar radiation, entropy, and resources
// @Tags environment
// @Produce json
// @Success 200 {object} response.Response{data=EnvironmentState}
// @Router /environment [get]
func (h *Handler) GetEnvironment(c *fiber.Ctx) error {
	env := h.service.GetEnvironment()
	return response.OK(c, env)
}

// TriggerEvent injects an environmental intervention into the sandbox.
// @Summary Trigger environmental event
// @Description Apply solar flare, cryo wave, or EM pulse to the system
// @Tags environment
// @Accept json
// @Produce json
// @Param request body TriggerEventRequest true "Event parameters"
// @Success 200 {object} response.Response{data=EnvironmentState}
// @Router /environment/trigger [post]
func (h *Handler) TriggerEvent(c *fiber.Ctx) error {
	var req TriggerEventRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	env, err := h.service.TriggerEvent(req)
	if err != nil {
		return response.InternalError(c, err.Error())
	}
	return response.OK(c, env)
}

// ResetSimulation restores the initial baseline state.
// @Summary Reset simulation
// @Description Reset all colonies and environment to initial state
// @Tags simulation
// @Produce json
// @Success 200 {object} response.Response
// @Router /simulation/reset [post]
func (h *Handler) ResetSimulation(c *fiber.Ctx) error {
	h.service.ResetSimulation()
	return response.OK(c, fiber.Map{"message": "Simulation reset successfully"})
}

// StepSimulation executes a single time step of the simulation.
// @Summary Step simulation
// @Description Advance simulation by 1 tick
// @Tags simulation
// @Produce json
// @Success 200 {object} response.Response
// @Router /simulation/step [post]
func (h *Handler) StepSimulation(c *fiber.Ctx) error {
	h.service.StepSimulation()
	return response.OK(c, fiber.Map{
		"colonies":    h.service.GetColonies(),
		"environment": h.service.GetEnvironment(),
	})
}

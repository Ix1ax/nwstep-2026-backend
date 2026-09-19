package transport

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
	"github.com/rs/zerolog"
)

func (h *Handler) Compare(c *fiber.Ctx) error {
	exp, err := h.manager.GetExperiment(c.Params("id"))
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	source := exp.View()
	ticks := c.QueryInt("ticks", 300)
	if ticks < 1 || ticks > 600 {
		return response.BadRequest(c, "comparison ticks must be 1..600")
	}
	results := make([]fiber.Map, 0, 3)
	for _, mode := range []model.Mode{model.ModeReactive, model.ModeAdaptive, model.ModeEvolutionary} {
		run := experiments.NewManager(zerolog.Nop()).CreateExperiment("comparison", source.WorldID, mode, source.Seed, nil)
		run.Interventions = nil
		for _, it := range source.Interventions {
			if it.Type != "set_mode" && it.Type != "toggle_mutations" {
				run.Interventions = append(run.Interventions, it)
			}
		}
		snapshot, err := run.Replay(int64(ticks))
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		results = append(results, fiber.Map{"mode": mode, "metrics": snapshot.Metrics, "history": run.View().MetricsHistory})
	}
	return response.OK(c, results)
}

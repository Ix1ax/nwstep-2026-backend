package transport

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/export"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type importJob struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Tick         int64  `json:"tick"`
	Total        int64  `json:"total"`
	ExperimentID string `json:"experimentId,omitempty"`
	Error        string `json:"error,omitempty"`
	cancel       context.CancelFunc
	created      time.Time
}

// ImportExperiment returns immediately; expensive deterministic validation runs
// privately, with bounded concurrency. Incomplete experiments are never visible.
func (h *Handler) ImportExperiment(c *fiber.Ctx) error {
	bundle, err := export.ImportJSON(c.Body())
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	h.importsMu.Lock()
	for id, job := range h.imports {
		if job.Status != "running" && time.Since(job.created) > 30*time.Minute {
			delete(h.imports, id)
		}
	}
	if len(h.imports) >= 100 {
		h.importsMu.Unlock()
		return response.Error(c, 429, "IMPORT_LIMIT", "Too many imports")
	}
	select {
	case h.importSlots <- struct{}{}:
	default:
		h.importsMu.Unlock()
		return response.Error(c, 429, "IMPORT_BUSY", "Another recording is being restored; try again shortly")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	job := &importJob{ID: uuid.NewString(), Status: "running", Total: bundle.FinalSnapshot.Tick, cancel: cancel, created: time.Now()}
	h.imports[job.ID] = job
	initial := *job
	h.importsMu.Unlock()
	go func() {
		defer cancel()
		defer func() { <-h.importSlots }()
		seed, _ := strconv.ParseUint(bundle.Seed, 10, 64)
		exp := experiments.NewManager(h.log).CreateExperiment(bundle.Name, bundle.World.ID, bundle.Mode, seed, nil)
		exp.Parameters = bundle.Parameters
		exp.Interventions = bundle.Interventions
		snap, err := exp.ReplayContext(ctx, bundle.FinalSnapshot.Tick, func(tick int64) {
			h.importsMu.Lock()
			job.Tick = tick
			h.importsMu.Unlock()
		})
		h.importsMu.Lock()
		defer h.importsMu.Unlock()
		if ctx.Err() != nil {
			job.Status = "cancelled"
			job.Error = "Восстановление отменено или превышено время ожидания"
			return
		}
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			return
		}
		if snap.Checksum != bundle.FinalSnapshot.Checksum {
			job.Status = "failed"
			job.Error = "Контрольная сумма не совпадает: запись повреждена или создана другой версией модели"
			return
		}
		if err = h.manager.Adopt(exp, h.BroadcastSnapshot); err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			return
		}
		job.Status = "completed"
		job.ExperimentID = exp.ID
	}()
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"success": true, "data": initial})
}

func (h *Handler) GetImport(c *fiber.Ctx) error {
	h.importsMu.Lock()
	defer h.importsMu.Unlock()
	job := h.imports[c.Params("id")]
	if job == nil {
		return response.NotFound(c, "Import not found")
	}
	return response.OK(c, job)
}
func (h *Handler) CancelImport(c *fiber.Ctx) error {
	h.importsMu.Lock()
	defer h.importsMu.Unlock()
	job := h.imports[c.Params("id")]
	if job == nil {
		return response.NotFound(c, "Import not found")
	}
	if job.Status == "running" {
		job.cancel()
	}
	return response.OK(c, job)
}

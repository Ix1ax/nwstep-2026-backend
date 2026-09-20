package transport

import (
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/behavior"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
	"github.com/rs/zerolog"
)

type demoFrame struct {
	Title       string                `json:"title"`
	Context     string                `json:"context"`
	Before      model.Individual      `json:"before"`
	After       model.Individual      `json:"after"`
	Decision    *model.DecisionTrace  `json:"decision"`
	Explanation *behavior.Explanation `json:"explanation"`
	Population  int                   `json:"population"`
}
type demoEpisode struct {
	Seed       uint64           `json:"seed"`
	Parameters model.Parameters `json:"parameters"`
	Frames     []demoFrame      `json:"frames"`
}

// Fixed, bounded, isolated run. No user experiments, timers, or checkpoint writes.
func buildDemo() (*demoEpisode, error) {
	mgr := experiments.NewManager(zerolog.Nop())
	exp := mgr.CreateExperiment("Narrated demo", "earth", model.ModeAdaptive, 2048, nil)
	exp.State.Interventions = append(exp.State.Interventions, &model.Intervention{ID: "demo-flow", Tick: 125, Type: "set_flow", TargetID: "earth", Value: 0})
	episode := &demoEpisode{Seed: 2048, Parameters: exp.Parameters, Frames: []demoFrame{}}
	chapters := map[int64][2]string{
		1:   {"Ресурс превращается в структуру", "Среда даёт энергию. Особь сравнивает сохранение, помощь соседу, рост и деление."},
		94:  {"Приближение к делению", "Структура накапливается постепенно. Проверим, достаточно ли её для появления потомка."},
		100: {"Проверка готовности", "Для деления должны одновременно выполняться условия по структуре, энергии и времени."},
		145: {"Среда перестала давать ресурс", "На такте 125 сценарий автоматически отключил внешний приток. Память постепенно отражает дефицит и меняет приоритеты."},
		180: {"Решение после изменения среды", "Тот же алгоритм оценивает уже другое состояние. Сравните память, запас и веса с началом показа."},
	}
	for tick := int64(1); tick <= 180; tick++ {
		var captured *demoFrame
		if chapter, ok := chapters[tick]; ok {
			exp.Engine.ObserveDecision = func(ind model.Individual, trace *model.DecisionTrace, detail *behavior.Explanation) {
				if ind.ID != "ind-01" {
					return
				}
				ind.LastDecision = nil
				captured = &demoFrame{Title: chapter[0], Context: chapter[1], Before: ind, Decision: trace, Explanation: detail}
			}
		} else {
			exp.Engine.ObserveDecision = nil
		}
		if err := exp.SendCommand("step", 1); err != nil {
			return nil, err
		}
		if captured != nil {
			captured.After = *exp.State.Individuals[captured.Before.ID]
			captured.After.LastDecision = nil
			captured.Population = exp.LatestSnapshot.Metrics.Population
			episode.Frames = append(episode.Frames, *captured)
		}
	}
	return episode, nil
}

var demoCache struct {
	sync.Once
	episode *demoEpisode
	err     error
}

func (h *Handler) GetDemo(c *fiber.Ctx) error {
	demoCache.Do(func() { demoCache.episode, demoCache.err = buildDemo() })
	if demoCache.err != nil {
		return response.InternalError(c, "Не удалось подготовить демо")
	}
	return response.OK(c, demoCache.episode)
}

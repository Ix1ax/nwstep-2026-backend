package environment_test

import (
	"testing"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

func TestEnvironmentModules_AllWorlds(t *testing.T) {
	worlds := model.GetPresetWorlds()
	if len(worlds) != 3 {
		t.Fatalf("Expected 3 preset worlds, got %d", len(worlds))
	}

	for _, w := range worlds {
		env := environment.NewEnvironmentModule(w)
		if env.WorldID() != w.ID {
			t.Fatalf("WorldID mismatch: expected %s, got %s", w.ID, env.WorldID())
		}

		ind := &model.Individual{
			ID:       "ind-test",
			WorldID:  w.ID,
			ColonyID: "col-1",
			Lat:      10.0,
			Lng:      20.0,
			Energy:   30.0,
			Biomass:  10.0,
		}

		// 1. Поле и приток энергии
		fieldVal := env.SampleField(ind.Lat, ind.Lng, 10)
		if fieldVal < 0 {
			t.Fatalf("Field value cannot be negative for world %s: %f", w.ID, fieldVal)
		}

		input := env.ResourceInput(ind, 0.1, 0.5)
		if input < 0 {
			t.Fatalf("Resource input cannot be negative for world %s: %f", w.ID, input)
		}

		// 2. Затраты на поддержание
		cost := env.MaintenanceCost(ind, 0.1)
		if cost <= 0 {
			t.Fatalf("Maintenance cost must be positive for world %s: %f", w.ID, cost)
		}

		// 3. Свойства канала
		indNeighbor := &model.Individual{
			ID:       "ind-neighbor",
			WorldID:  w.ID,
			ColonyID: "col-1",
			Lat:      10.5,
			Lng:      20.5,
		}
		dist, loss, delay := env.ChannelProperties(ind, indNeighbor)
		if dist <= 0 || loss <= 0 || loss > 0.9 || delay < 1 {
			t.Fatalf("Invalid channel properties for world %s: dist=%f, loss=%f, delay=%d",
				w.ID, dist, loss, delay)
		}

		// 4. Вмешательство
		env.ApplyIntervention(&model.Intervention{
			Type:  "set_flow",
			Value: 0.5,
		})
		if env.GetFlowMultiplier() != 0.5 {
			t.Fatalf("Expected flowMultiplier 0.5, got %f", env.GetFlowMultiplier())
		}
	}
}

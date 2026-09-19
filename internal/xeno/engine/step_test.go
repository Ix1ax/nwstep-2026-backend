package engine_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

func TestEngine_DeterminismAndChecksum_AllWorlds(t *testing.T) {
	worlds := []string{"earth", "mars", "venus"}
	seed := uint64(12345)

	for _, worldID := range worlds {
		t.Run(worldID, func(t *testing.T) {
			mgr1 := experiments.NewManager(zerolog.Nop())
			exp1 := mgr1.CreateExperiment("Run1", worldID, model.ModeEvolutionary, seed, nil)

			mgr2 := experiments.NewManager(zerolog.Nop())
			exp2 := mgr2.CreateExperiment("Run2", worldID, model.ModeEvolutionary, seed, nil)

			// Выполняем 30 тактов в обоих экспериментах
			for i := 0; i < 30; i++ {
				if err := exp1.SendCommand("step", 1); err != nil {
					t.Fatalf("exp1 step %d failed: %v", i, err)
				}
				if err := exp2.SendCommand("step", 1); err != nil {
					t.Fatalf("exp2 step %d failed: %v", i, err)
				}

				snap1 := exp1.LatestSnapshot
				snap2 := exp2.LatestSnapshot

				if snap1.Checksum != snap2.Checksum {
					t.Fatalf("Checksum mismatch at tick %d: %s vs %s", snap1.Tick, snap1.Checksum, snap2.Checksum)
				}

				if snap1.Metrics.BalanceResidual != snap2.Metrics.BalanceResidual {
					t.Fatalf("Balance residual mismatch at tick %d: %f vs %f", snap1.Tick, snap1.Metrics.BalanceResidual, snap2.Metrics.BalanceResidual)
				}

				// Закон сохранения энергии: невязка не должна превышать 1e-6
				if snap1.Metrics.BalanceResidual > 1e-6 {
					t.Fatalf("Energy conservation violated at tick %d: residual = %e", snap1.Tick, snap1.Metrics.BalanceResidual)
				}
			}
		})
	}
}

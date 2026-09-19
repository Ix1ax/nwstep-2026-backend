package experiments_test

import (
	"testing"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/rs/zerolog"
)

func TestBenchmarkScenario_AllWorldsAndReplay(t *testing.T) {
	params := model.DefaultParameters()
	seed := uint64(42)

	// 1. Проверка структуры сценариев на Земле, Марсе и Венере (раздел 4 и 5 ТЗ v2)
	worlds := []string{"earth", "mars", "venus"}
	for _, wID := range worlds {
		snap, interventions := experiments.BuildWorldScenario(wID, model.ModeEvolutionary, seed, params)
		if len(snap.Individuals) != 12 {
			t.Fatalf("World %s: expected 12 individuals, got %d", wID, len(snap.Individuals))
		}
		if len(snap.Colonies) != 2 {
			t.Fatalf("World %s: expected 2 colonies, got %d", wID, len(snap.Colonies))
		}
		if len(interventions) != 0 {
			t.Fatalf("World %s: expected 0 automatic interventions, got %d", wID, len(interventions))
		}
		if snap.World.ID != wID {
			t.Fatalf("World ID mismatch: expected %s, got %s", wID, snap.World.ID)
		}
	}

	// 2. Тестирование 3 режимов симуляции на Земле (раздел 7.3 ТЗ v2)
	mgr := experiments.NewManager(zerolog.Nop())
	expR := mgr.CreateExperiment("Earth-Reactive", "earth", model.ModeReactive, seed, nil)
	expA := mgr.CreateExperiment("Earth-Adaptive", "earth", model.ModeAdaptive, seed, nil)
	expE := mgr.CreateExperiment("Earth-Evolutionary", "earth", model.ModeEvolutionary, seed, nil)

	for i := 0; i < 30; i++ {
		if err := expR.SendCommand("step", 1); err != nil {
			t.Fatalf("Reactive step failed: %v", err)
		}
		if err := expA.SendCommand("step", 1); err != nil {
			t.Fatalf("Adaptive step failed: %v", err)
		}
		if err := expE.SendCommand("step", 1); err != nil {
			t.Fatalf("Evolutionary step failed: %v", err)
		}
	}

	for _, exp := range []*experiments.Experiment{expR, expA, expE} {
		if exp.LatestSnapshot.Tick != 30 { // такты 0..29
			t.Fatalf("Expected tick 30, got %d for mode %s", exp.LatestSnapshot.Tick, exp.Mode)
		}
		if exp.LatestSnapshot.Metrics.BalanceResidual > 1e-6 {
			t.Fatalf("Energy balance violated for mode %s: residual = %e",
				exp.Mode, exp.LatestSnapshot.Metrics.BalanceResidual)
		}
	}

	// 3. Тестирование Replay (раздел 14 ТЗ v2)
	// Сохраняем контрольную сумму после 30 тактов
	expectedChecksum := expE.LatestSnapshot.Checksum
	targetTick := int64(30)

	replayedSnap, err := expE.Replay(targetTick)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if replayedSnap.Checksum != expectedChecksum {
		t.Fatalf("Replay checksum mismatch: expected %s, got %s", expectedChecksum, replayedSnap.Checksum)
	}
}

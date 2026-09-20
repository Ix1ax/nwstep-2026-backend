package experiments_test

import (
	"context"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/rs/zerolog"
	"sync"
	"testing"
	"time"
)

func TestResearchLifecycle(t *testing.T) {
	for _, world := range []string{"earth", "mars", "venus"} {
		t.Run(world, func(t *testing.T) {
			e := experiments.NewManager(zerolog.Nop()).CreateExperiment("research", world, model.ModeEvolutionary, 42, nil)
			_, err := e.AddIntervention(model.Intervention{Type: "add_inoculum", Name: "Placed", Value: 45, Params: map[string]float64{"lat": 15, "lng": 80, "count": 6, "biomass": 10}})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 400; i++ {
				if err := e.SendCommand("step", 1); err != nil {
					t.Fatal(err)
				}
			}
			view := e.View()
			if len(view.LatestSnapshot.Colonies) < 3 {
				t.Fatal("colony not created")
			}
			t.Logf("%s births=%d population=%d balance=%g", world, view.LatestSnapshot.Metrics.BirthsTotal, view.LatestSnapshot.Metrics.Population, view.LatestSnapshot.Metrics.BalanceResidual)
			if view.LatestSnapshot.Metrics.BirthsTotal == 0 {
				t.Fatal("no reproduction")
			}
			if view.LatestSnapshot.Metrics.BalanceResidual > 1e-6 {
				t.Fatal("energy not conserved")
			}
			checksum := view.LatestSnapshot.Checksum
			snap, err := e.Replay(view.LatestSnapshot.Tick)
			if err != nil {
				t.Fatal(err)
			}
			if snap.Checksum != checksum {
				t.Fatal("replay differs")
			}
			_, err = e.Preview(10)
			if err != nil {
				t.Fatal(err)
			}
			if e.View().LatestSnapshot.Checksum != checksum {
				t.Fatal("preview mutated experiment")
			}
		})
	}
}
func TestTemporaryIntervention(t *testing.T) {
	e := experiments.NewManager(zerolog.Nop()).CreateExperiment("x", "earth", model.ModeAdaptive, 42, nil)
	it, err := e.AddIntervention(model.Intervention{Type: "depletion", Duration: 3})
	if err != nil {
		t.Fatal(err)
	}
	if it.Tick != 1 {
		t.Fatal("wrong tick")
	}
	for i := 0; i < 4; i++ {
		e.SendCommand("step", 1)
		want := 0.0
		if i == 3 {
			want = 1
		}
		if got := e.View().LatestSnapshot.Flow; got != want {
			t.Fatalf("step %d flow=%g", i, got)
		}
	}
}

func TestConcurrentReadersAndPreview(t *testing.T) {
	e := experiments.NewManager(zerolog.Nop()).CreateExperiment("concurrent", "earth", model.ModeEvolutionary, 42, nil)
	if err := e.SendCommand("start", 5); err != nil {
		t.Fatal(err)
	}
	defer e.SendCommand("pause", 1)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				v := e.View()
				if _, err := e.Preview(v.LatestSnapshot.Tick); err != nil {
					t.Error(err)
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}
	wg.Wait()
}

func TestReplayCancellationAndCompletionLimit(t *testing.T) {
	e := experiments.NewManager(zerolog.Nop()).CreateExperiment("cancel", "earth", model.ModeEvolutionary, 42, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.ReplayContext(ctx, 2000, nil); err == nil {
		t.Fatal("cancel ignored")
	}
	if e.View().LatestSnapshot.Tick != 0 {
		t.Fatal("cancel changed live state")
	}
	start := time.Now()
	if _, err := e.Replay(2000); err != nil {
		t.Fatal(err)
	}
	t.Logf("2000 ticks restored in %s", time.Since(start))
	if v := e.View(); v.Status != model.StatusCompleted || v.LatestSnapshot.Status != model.StatusCompleted {
		t.Fatal("completed replay must report completed status")
	}
	if err := e.SendCommand("step", 1); err == nil {
		t.Fatal("stepped beyond replay limit")
	}
	if err := e.SendCommand("resume", 1); err == nil {
		t.Fatal("resumed completed experiment")
	}
}

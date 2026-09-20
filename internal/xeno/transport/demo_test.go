package transport

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/rs/zerolog"
	"math"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

func TestDemoIsDeterministicAndExplainsActualScores(t *testing.T) {
	a, err := buildDemo()
	if err != nil {
		t.Fatal(err)
	}
	b, err := buildDemo()
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		t.Fatal("demo must be reproducible")
	}
	if len(a.Frames) != 5 {
		t.Fatalf("expected 5 chapters, got %d", len(a.Frames))
	}
	for _, f := range a.Frames {
		t.Logf("tick %d action %s E %.3f B %.3f M %.3f", f.Decision.Tick, f.Decision.SelectedAction, f.Before.Energy, f.Before.Biomass, f.Before.Memory)
		for action, score := range f.Decision.Scores {
			terms, ok := f.Explanation.Terms[action]
			if !ok {
				if f.Explanation.Blocked[action] == "" || score != -1 {
					t.Fatalf("missing blocked explanation for %s", action)
				}
				continue
			}
			sum := 0.
			for _, term := range terms {
				sum += term
			}
			if math.Abs(sum-score) > 0.00000051 {
				t.Fatalf("terms disagree with actual score %s: %v != %v", action, sum, score)
			}
		}
		if f.Decision.SelectedAction != model.ActionStore && f.Decision.ChosenScore-f.Decision.Scores[model.ActionStore] < f.Before.Genome.HThreshold-0.000001 {
			t.Fatal("choice does not pass threshold")
		}
	}
	if a.Frames[3].Before.Memory >= 0 {
		t.Fatal("depletion chapter must demonstrate negative memory")
	}
}

func TestDemoDoesNotChangeNormalSimulation(t *testing.T) {
	episode, err := buildDemo()
	if err != nil {
		t.Fatal(err)
	}
	mgr := experiments.NewManager(zerolog.Nop())
	exp := mgr.CreateExperiment("Control", "earth", model.ModeAdaptive, 2048, nil)
	exp.State.Interventions = append(exp.State.Interventions, &model.Intervention{ID: "demo-flow", Tick: 125, Type: "set_flow", TargetID: "earth", Value: 0})
	frame := 0
	for tick := int64(1); tick <= 180; tick++ {
		if err := exp.SendCommand("step", 1); err != nil {
			t.Fatal(err)
		}
		if frame < len(episode.Frames) && episode.Frames[frame].Decision.Tick == tick {
			actual := *exp.State.Individuals["ind-01"]
			actual.LastDecision = nil
			want := episode.Frames[frame].After
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("instrumentation changes individual at tick %d", tick)
			}
			frame++
		}
	}
	app := fiber.New()
	handler := NewHandler(mgr, zerolog.Nop())
	app.Get("/demo", handler.GetDemo)
	before := exp.LatestSnapshot.Checksum
	for i := 0; i < 2; i++ {
		res, err := app.Test(httptest.NewRequest("GET", "/demo", nil))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("HTTP %d", res.StatusCode)
		}
		res.Body.Close()
	}
	if len(mgr.ListExperiments()) != 1 || exp.LatestSnapshot.Checksum != before {
		t.Fatal("viewing demo changed user experiments")
	}
}

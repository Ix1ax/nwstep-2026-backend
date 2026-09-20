package experiments

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/rs/zerolog"
)

func TestCheckpointRoundTripAndAtomicFailure(t *testing.T) {
	source := NewManager(zerolog.Nop())
	exp := source.CreateExperiment("checkpoint", "mars", model.ModeEvolutionary, 42, nil)
	if _, err := exp.AddIntervention(model.Intervention{Type: "depletion", Duration: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := exp.Replay(20); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := source.Persist(dir); err != nil {
		t.Fatal(err)
	}
	restored := NewManager(zerolog.Nop())
	if err := restored.Restore(dir, nil); err != nil {
		t.Fatal(err)
	}
	got, err := restored.GetExperiment(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.View().LatestSnapshot.Checksum != exp.View().LatestSnapshot.Checksum {
		t.Fatal("restored checksum differs")
	}
	for _, e := range []*Experiment{exp, got} {
		if err := e.SendCommand("step", 1); err != nil {
			t.Fatal(err)
		}
	}
	if got.View().LatestSnapshot.Checksum != exp.View().LatestSnapshot.Checksum {
		t.Fatal("continuation differs")
	}

	valid := exp.View()
	invalid := exp.View()
	invalid.ID = "corrupted"
	invalid.LatestSnapshot.Checksum = "wrong"
	data, err := json.Marshal(persisted{Version: "surface-ecology-3.0", Experiments: []*Experiment{valid, invalid}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "research.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	target := NewManager(zerolog.Nop())
	existing := target.CreateExperiment("keep", "earth", model.ModeAdaptive, 7, nil)
	if err := target.Restore(dir, nil); err == nil {
		t.Fatal("corrupt checkpoint accepted")
	}
	if list := target.ListExperiments(); len(list) != 1 || list[0].ID != existing.ID {
		t.Fatal("failed restore published partial experiments")
	}
}

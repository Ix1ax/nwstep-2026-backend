package experiments

import (
	"encoding/json"
	"fmt"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"os"
	"path/filepath"
	"time"
)

type persisted struct {
	Version     string        `json:"version"`
	Experiments []*Experiment `json:"experiments"`
}

// Persist writes one atomic, crash-safe checkpoint. No credentials are stored.
func (m *Manager) Persist(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	p := persisted{Version: "surface-ecology-3.0"}
	for _, e := range m.ListExperiments() {
		p.Experiments = append(p.Experiments, e.View())
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "checkpoint-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "research.json"))
}
func (m *Manager) Restore(dir string, broadcast func(*model.StateSnapshot, string)) error {
	data, err := os.ReadFile(filepath.Join(dir, "research.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var p persisted
	if err = json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Version != "surface-ecology-3.0" {
		return fmt.Errorf("unsupported checkpoint model")
	}
	for _, saved := range p.Experiments {
		if saved == nil || saved.LatestSnapshot == nil || saved.LatestSnapshot.Tick > 2000 || !ValidMode(saved.Mode) {
			return fmt.Errorf("invalid checkpoint")
		}
		exp := m.CreateExperiment(saved.Name, saved.WorldID, saved.Mode, saved.Seed, broadcast)
		exp.Parameters = saved.Parameters
		exp.Interventions = saved.Interventions
		snap, err := exp.Replay(saved.LatestSnapshot.Tick)
		if err != nil {
			return err
		}
		if saved.LatestSnapshot.Tick > 0 && snap.Checksum != saved.LatestSnapshot.Checksum {
			return fmt.Errorf("checkpoint mismatch for %s", saved.ID)
		}
		m.mu.Lock()
		delete(m.experiments, exp.ID)
		exp.ID = saved.ID
		exp.CreatedAt = saved.CreatedAt
		m.experiments[exp.ID] = exp
		m.mu.Unlock()
	}
	return nil
}
func (m *Manager) StartCheckpoints(dir string) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := m.Persist(dir); err != nil {
				m.log.Error().Err(err).Msg("Research checkpoint failed")
			}
		}
	}()
}

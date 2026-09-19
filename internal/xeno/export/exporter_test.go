package export_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/export"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

func TestExporter_JSONAndCSV(t *testing.T) {
	mgr := experiments.NewManager(zerolog.Nop())
	exp := mgr.CreateExperiment("ExportTest", "mars", model.ModeAdaptive, 42, nil)

	// Делаем 5 тактов
	for i := 0; i < 5; i++ {
		_ = exp.SendCommand("step", 1)
	}

	// 1. JSON Export & Import
	jsonData, err := export.ExportJSON(exp)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}
	if len(jsonData) == 0 {
		t.Fatalf("ExportJSON returned empty data")
	}

	bundle, err := export.ImportJSON(jsonData)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}
	if bundle.ExperimentID != exp.ID {
		t.Fatalf("Imported bundle ID mismatch: %s vs %s", bundle.ExperimentID, exp.ID)
	}
	if bundle.World.ID != "mars" {
		t.Fatalf("Imported bundle world ID mismatch: %s vs mars", bundle.World.ID)
	}

	// 2. CSV Export
	csvData, err := export.ExportCSV(exp)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}
	if len(csvData) == 0 {
		t.Fatalf("ExportCSV returned empty data")
	}
}

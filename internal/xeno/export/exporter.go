package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"strconv"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

const (
	SchemaVersionV2 = "3.0.0"
	ModelVersionV2  = "surface-ecology-3.0"
)

// ExportBundle — полный формат JSON-экспорта согласно разделу 14 ТЗ v2
type ExportBundle struct {
	SchemaVersion  string                   `json:"schemaVersion"`
	ModelVersion   string                   `json:"modelVersion"`
	PRNG           string                   `json:"prng"`
	ExperimentID   string                   `json:"experimentId"`
	Name           string                   `json:"name"`
	World          model.World              `json:"world"`
	Mode           model.Mode               `json:"mode"`
	Status         model.ExperimentStatus   `json:"status"`
	Seed           string                   `json:"seed"` // Десятичная строка
	Parameters     model.Parameters         `json:"parameters"`
	Interventions  []*model.Intervention    `json:"interventions"`
	FinalSnapshot  *model.StateSnapshot     `json:"finalSnapshot"`
	MetricsHistory []*model.MetricsSnapshot `json:"metricsHistory"`
}

// ExportJSON формирует канонический JSON-дамп эксперимента
func ExportJSON(exp *experiments.Experiment) ([]byte, error) {
	exp = exp.View()
	bundle := ExportBundle{
		SchemaVersion:  SchemaVersionV2,
		ModelVersion:   ModelVersionV2,
		PRNG:           "SplitMix64-v1",
		ExperimentID:   exp.ID,
		Name:           exp.Name,
		World:          exp.LatestSnapshot.World,
		Mode:           exp.Mode,
		Status:         exp.Status,
		Seed:           strconv.FormatUint(exp.Seed, 10),
		Parameters:     exp.Parameters,
		Interventions:  exp.Interventions,
		FinalSnapshot:  exp.LatestSnapshot,
		MetricsHistory: exp.MetricsHistory,
	}

	return json.MarshalIndent(bundle, "", "  ")
}

// ExportCSV формирует CSV-таблицу метрик прогона (раздел 14 ТЗ v2)
// CSV: такт, численность особей, число колоний, рождения, смерти, входная мощность, использование ресурса, энтропия, задержка доставки, задержка ответа
func ExportCSV(exp *experiments.Experiment) ([]byte, error) {
	exp = exp.View()
	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	header := []string{
		"tick",
		"population",
		"colonies",
		"births",
		"colony_splits",
		"deaths",
		"input_power",
		"useful_power",
		"efficiency_percent",
		"decision_entropy_bits",
		"delivery_latency",
		"response_latency",
		"balance_residual",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, m := range exp.MetricsHistory {
		row := []string{
			strconv.FormatInt(m.Tick, 10),
			strconv.Itoa(m.Population),
			strconv.Itoa(m.ActiveColonies),
			strconv.Itoa(m.BirthsTotal),
			strconv.Itoa(m.ColonySplitsTotal),
			strconv.Itoa(m.DeathsTotal),
			fmt.Sprintf("%.4f", m.InputPower),
			fmt.Sprintf("%.4f", m.UsefulPower),
			fmt.Sprintf("%.2f", m.Efficiency),
			fmt.Sprintf("%.4f", m.DecisionEntropy),
			fmt.Sprintf("%.2f", m.DeliveryLatency),
			fmt.Sprintf("%.2f", m.ResponseLatency),
			fmt.Sprintf("%.6e", m.BalanceResidual),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ImportJSON проверяет и валидирует импортируемый JSON-эксперимент
func ImportJSON(data []byte) (*ExportBundle, error) {
	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("invalid json format: %w", err)
	}

	if bundle.SchemaVersion != SchemaVersionV2 {
		return nil, fmt.Errorf("incompatible schema version: got %s, expected %s",
			bundle.SchemaVersion, SchemaVersionV2)
	}

	if bundle.ModelVersion != ModelVersionV2 || bundle.PRNG != "SplitMix64-v1" {
		return nil, fmt.Errorf("incompatible simulation model")
	}
	if bundle.FinalSnapshot == nil || bundle.FinalSnapshot.Tick < 0 || bundle.FinalSnapshot.Tick > 2000 {
		return nil, fmt.Errorf("invalid final snapshot")
	}
	if _, ok := model.FindWorldByID(bundle.World.ID); !ok {
		return nil, fmt.Errorf("unknown world")
	}
	if !experiments.ValidMode(bundle.Mode) || len(bundle.Interventions) > 1000 {
		return nil, fmt.Errorf("invalid recording")
	}
	// Parameters must match this model version; never execute arbitrary imported coefficients.
	if bundle.Parameters != model.DefaultParameters() {
		return nil, fmt.Errorf("unsupported model parameters")
	}
	seed, err := strconv.ParseUint(bundle.Seed, 10, 64)
	if err != nil || seed == 0 {
		return nil, fmt.Errorf("invalid seed")
	}
	validator := experiments.NewManager(zerolog.Nop()).CreateExperiment("validation", bundle.World.ID, bundle.Mode, seed, nil)
	lastTick := int64(0)
	for _, it := range bundle.Interventions {
		if it == nil || it.Tick < 1 || it.Tick < lastTick || it.Tick > bundle.FinalSnapshot.Tick+1 {
			return nil, fmt.Errorf("invalid event timeline")
		}
		lastTick = it.Tick
		copyEvent := *it
		if it.Genome != nil {
			g := *it.Genome
			copyEvent.Genome = &g
		}
		if _, err := validator.AddIntervention(copyEvent); err != nil {
			return nil, err
		}
	}
	return &bundle, nil
}

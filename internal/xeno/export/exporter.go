package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

const (
	SchemaVersionV2 = "2.0.0"
	ModelVersionV2  = "surface-ecology-2.0"
)

// ExportBundle — полный формат JSON-экспорта согласно разделу 14 ТЗ v2
type ExportBundle struct {
	SchemaVersion   string                   `json:"schemaVersion"`
	ModelVersion    string                   `json:"modelVersion"`
	PRNG            string                   `json:"prng"`
	ExperimentID    string                   `json:"experimentId"`
	Name            string                   `json:"name"`
	World           model.World              `json:"world"`
	Mode            model.Mode               `json:"mode"`
	Status          model.ExperimentStatus   `json:"status"`
	Seed            string                   `json:"seed"` // Десятичная строка
	Parameters      model.Parameters         `json:"parameters"`
	Interventions   []*model.Intervention    `json:"interventions"`
	FinalSnapshot   *model.StateSnapshot     `json:"finalSnapshot"`
	MetricsHistory  []*model.MetricsSnapshot `json:"metricsHistory"`
}

// ExportJSON формирует канонический JSON-дамп эксперимента
func ExportJSON(exp *experiments.Experiment) ([]byte, error) {
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

	if bundle.SchemaVersion != SchemaVersionV2 && bundle.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("incompatible schema version: got %s, expected %s",
			bundle.SchemaVersion, SchemaVersionV2)
	}

	return &bundle, nil
}

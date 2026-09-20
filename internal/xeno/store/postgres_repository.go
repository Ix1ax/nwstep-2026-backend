package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

var (
	ErrNotFound = errors.New("record not found in database")
)

type PersistedExperiment struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	WorldID         string                   `json:"worldId"`
	Mode            model.Mode               `json:"mode"`
	Status          model.ExperimentStatus   `json:"status"`
	Seed            uint64                   `json:"seed"`
	Speed           int                      `json:"speed"`
	Parameters      model.Parameters         `json:"parameters"`
	Interventions   []*model.Intervention    `json:"interventions"`
	InitialSnapshot *model.StateSnapshot     `json:"initialSnapshot"`
	LatestSnapshot  *model.StateSnapshot     `json:"latestSnapshot"`
	MetricsHistory  []*model.MetricsSnapshot `json:"metricsHistory"`
	CreatedAt       time.Time                `json:"createdAt"`
	UpdatedAt       time.Time                `json:"updatedAt"`
}

type PostgresRepository struct {
	db  *sqlx.DB
	log zerolog.Logger
}

func NewPostgresRepository(db *sqlx.DB, log zerolog.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:  db,
		log: log.With().Str("component", "postgres_repo").Logger(),
	}
}

// DB returns the underlying sqlx.DB connection
func (r *PostgresRepository) DB() *sqlx.DB {
	return r.db
}

// SaveExperiment creates or updates the full experiment record in PostgreSQL
func (r *PostgresRepository) SaveExperiment(ctx context.Context, exp *PersistedExperiment) error {
	if r.db == nil {
		return nil
	}

	paramsJSON, err := json.Marshal(exp.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	initSnapJSON, err := json.Marshal(exp.InitialSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal initial_snapshot: %w", err)
	}

	latestSnapJSON, err := json.Marshal(exp.LatestSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal latest_snapshot: %w", err)
	}

	metricsJSON, err := json.Marshal(exp.MetricsHistory)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics_history: %w", err)
	}

	interventionsJSON, err := json.Marshal(exp.Interventions)
	if err != nil {
		return fmt.Errorf("failed to marshal interventions: %w", err)
	}

	query := `
		INSERT INTO experiments (
			id, name, world_id, mode, status, seed, speed,
			parameters, initial_snapshot, latest_snapshot, metrics_history, interventions,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			speed = EXCLUDED.speed,
			latest_snapshot = EXCLUDED.latest_snapshot,
			metrics_history = EXCLUDED.metrics_history,
			interventions = EXCLUDED.interventions,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	createdAt := exp.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	_, err = r.db.ExecContext(ctx, query,
		exp.ID, exp.Name, exp.WorldID, string(exp.Mode), string(exp.Status), exp.Seed, exp.Speed,
		paramsJSON, initSnapJSON, latestSnapJSON, metricsJSON, interventionsJSON,
		createdAt, now,
	)
	if err != nil {
		r.log.Error().Err(err).Str("expId", exp.ID).Msg("Failed to upsert experiment in PostgreSQL")
		return err
	}

	r.log.Debug().Str("expId", exp.ID).Msg("Saved experiment to PostgreSQL")
	return nil
}

// UpdateExperimentState performs a targeted update of runtime experiment state
func (r *PostgresRepository) UpdateExperimentState(
	ctx context.Context,
	id string,
	status model.ExperimentStatus,
	latestSnapshot *model.StateSnapshot,
	metricsHistory []*model.MetricsSnapshot,
	speed int,
) error {
	if r.db == nil {
		return nil
	}

	latestSnapJSON, err := json.Marshal(latestSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal latest_snapshot: %w", err)
	}

	metricsJSON, err := json.Marshal(metricsHistory)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics_history: %w", err)
	}

	query := `
		UPDATE experiments SET
			status = $2,
			speed = $3,
			latest_snapshot = $4,
			metrics_history = $5,
			updated_at = $6
		WHERE id = $1
	`

	_, err = r.db.ExecContext(ctx, query, id, string(status), speed, latestSnapJSON, metricsJSON, time.Now())
	if err != nil {
		r.log.Error().Err(err).Str("expId", id).Msg("Failed to update experiment state in PostgreSQL")
		return err
	}

	return nil
}

// SaveIntervention persists an individual intervention event to the relational audit log and the experiment
func (r *PostgresRepository) SaveIntervention(ctx context.Context, expID string, it *model.Intervention) error {
	if r.db == nil {
		return nil
	}

	paramsJSON, _ := json.Marshal(it.Params)
	if len(paramsJSON) == 0 || string(paramsJSON) == "null" {
		paramsJSON = []byte("{}")
	}

	var genomeParam interface{} = nil
	if it.Genome != nil {
		gBytes, err := json.Marshal(it.Genome)
		if err == nil {
			genomeParam = string(gBytes)
		}
	}

	query := `
		INSERT INTO interventions (
			id, experiment_id, tick, sequence, type, target_id,
			value, duration, name, color, genome, params, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, NOW()
		)
		ON CONFLICT (id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		it.ID, expID, it.Tick, it.Sequence, it.Type, it.TargetID,
		it.Value, it.Duration, it.Name, it.Color, genomeParam, string(paramsJSON),
	)
	if err != nil {
		r.log.Error().Err(err).Str("expId", expID).Str("interventionId", it.ID).Msg("Failed to insert intervention in PostgreSQL")
		return err
	}

	return nil
}

// SaveMetricsSnapshot inserts an analytics row for time-series analysis
func (r *PostgresRepository) SaveMetricsSnapshot(ctx context.Context, expID string, m *model.MetricsSnapshot) error {
	if r.db == nil || m == nil {
		return nil
	}

	dataJSON, _ := json.Marshal(m)

	query := `
		INSERT INTO simulation_metrics (
			experiment_id, tick, population, active_colonies, survival_rate,
			efficiency, decision_entropy, births_total, deaths_total, metrics_data, recorded_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, NOW()
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		expID, m.Tick, m.Population, m.ActiveColonies, m.SurvivalRate,
		m.Efficiency, m.DecisionEntropy, m.BirthsTotal, m.DeathsTotal, dataJSON,
	)
	if err != nil {
		r.log.Debug().Err(err).Str("expId", expID).Int64("tick", m.Tick).Msg("Could not insert metrics snapshot")
		return err
	}

	return nil
}

// GetExperiment loads a single experiment from PostgreSQL
func (r *PostgresRepository) GetExperiment(ctx context.Context, id string) (*PersistedExperiment, error) {
	if r.db == nil {
		return nil, ErrNotFound
	}

	query := `
		SELECT
			id, name, world_id, mode, status, seed, speed,
			parameters, initial_snapshot, latest_snapshot, metrics_history, interventions,
			created_at, updated_at
		FROM experiments
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var (
		expID             string
		name              string
		worldID           string
		modeStr           string
		statusStr         string
		seed              int64
		speed             int
		paramsBytes       []byte
		initSnapBytes     []byte
		latestSnapBytes   []byte
		metricsBytes      []byte
		interventionsBytes []byte
		createdAt         time.Time
		updatedAt         time.Time
	)

	err := row.Scan(
		&expID, &name, &worldID, &modeStr, &statusStr, &seed, &speed,
		&paramsBytes, &initSnapBytes, &latestSnapBytes, &metricsBytes, &interventionsBytes,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan experiment: %w", err)
	}

	exp := &PersistedExperiment{
		ID:        expID,
		Name:      name,
		WorldID:   worldID,
		Mode:      model.Mode(modeStr),
		Status:    model.ExperimentStatus(statusStr),
		Seed:      uint64(seed),
		Speed:     speed,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if len(paramsBytes) > 0 {
		_ = json.Unmarshal(paramsBytes, &exp.Parameters)
	}
	if len(initSnapBytes) > 0 {
		var snap model.StateSnapshot
		if err := json.Unmarshal(initSnapBytes, &snap); err == nil {
			exp.InitialSnapshot = &snap
		}
	}
	if len(latestSnapBytes) > 0 {
		var snap model.StateSnapshot
		if err := json.Unmarshal(latestSnapBytes, &snap); err == nil {
			exp.LatestSnapshot = &snap
		}
	}
	if len(metricsBytes) > 0 {
		_ = json.Unmarshal(metricsBytes, &exp.MetricsHistory)
	}
	if len(interventionsBytes) > 0 {
		_ = json.Unmarshal(interventionsBytes, &exp.Interventions)
	}

	return exp, nil
}

// ListExperiments returns all experiments stored in PostgreSQL
func (r *PostgresRepository) ListExperiments(ctx context.Context) ([]*PersistedExperiment, error) {
	if r.db == nil {
		return nil, nil
	}

	query := `
		SELECT
			id, name, world_id, mode, status, seed, speed,
			parameters, initial_snapshot, latest_snapshot, metrics_history, interventions,
			created_at, updated_at
		FROM experiments
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query experiments: %w", err)
	}
	defer rows.Close()

	var result []*PersistedExperiment
	for rows.Next() {
		var (
			expID             string
			name              string
			worldID           string
			modeStr           string
			statusStr         string
			seed              int64
			speed             int
			paramsBytes       []byte
			initSnapBytes     []byte
			latestSnapBytes   []byte
			metricsBytes      []byte
			interventionsBytes []byte
			createdAt         time.Time
			updatedAt         time.Time
		)

		if err := rows.Scan(
			&expID, &name, &worldID, &modeStr, &statusStr, &seed, &speed,
			&paramsBytes, &initSnapBytes, &latestSnapBytes, &metricsBytes, &interventionsBytes,
			&createdAt, &updatedAt,
		); err != nil {
			continue
		}

		exp := &PersistedExperiment{
			ID:        expID,
			Name:      name,
			WorldID:   worldID,
			Mode:      model.Mode(modeStr),
			Status:    model.ExperimentStatus(statusStr),
			Seed:      uint64(seed),
			Speed:     speed,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		if len(paramsBytes) > 0 {
			_ = json.Unmarshal(paramsBytes, &exp.Parameters)
		}
		if len(initSnapBytes) > 0 {
			var snap model.StateSnapshot
			if err := json.Unmarshal(initSnapBytes, &snap); err == nil {
				exp.InitialSnapshot = &snap
			}
		}
		if len(latestSnapBytes) > 0 {
			var snap model.StateSnapshot
			if err := json.Unmarshal(latestSnapBytes, &snap); err == nil {
				exp.LatestSnapshot = &snap
			}
		}
		if len(metricsBytes) > 0 {
			_ = json.Unmarshal(metricsBytes, &exp.MetricsHistory)
		}
		if len(interventionsBytes) > 0 {
			_ = json.Unmarshal(interventionsBytes, &exp.Interventions)
		}

		result = append(result, exp)
	}

	return result, nil
}

// DeleteExperiment removes the experiment and all cascaded data from PostgreSQL
func (r *PostgresRepository) DeleteExperiment(ctx context.Context, id string) error {
	if r.db == nil {
		return nil
	}

	_, err := r.db.ExecContext(ctx, "DELETE FROM experiments WHERE id = $1", id)
	if err != nil {
		r.log.Error().Err(err).Str("expId", id).Msg("Failed to delete experiment from PostgreSQL")
		return err
	}

	r.log.Info().Str("expId", id).Msg("Deleted experiment from PostgreSQL")
	return nil
}

// SaveChoiceLog records v1 dilemma choices for persistence
func (r *PostgresRepository) SaveChoiceLog(ctx context.Context, dilemmaID, altID string, entropyDelta float64, energyDeltas map[string]float64) error {
	if r.db == nil {
		return nil
	}

	deltasJSON, _ := json.Marshal(energyDeltas)
	query := `
		INSERT INTO choice_logs (dilemma_id, selected_alternative, entropy_delta, energy_deltas, status, created_at)
		VALUES ($1, $2, $3, $4, 'completed', NOW())
	`
	_, err := r.db.ExecContext(ctx, query, dilemmaID, altID, entropyDelta, deltasJSON)
	return err
}

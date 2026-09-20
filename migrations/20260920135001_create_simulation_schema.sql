-- +goose Up
CREATE TABLE IF NOT EXISTS experiments (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    world_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    seed BIGINT NOT NULL,
    speed INT NOT NULL DEFAULT 1,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    initial_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    latest_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    metrics_history JSONB NOT NULL DEFAULT '[]'::jsonb,
    interventions JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experiments_status ON experiments(status);
CREATE INDEX IF NOT EXISTS idx_experiments_world ON experiments(world_id);
CREATE INDEX IF NOT EXISTS idx_experiments_created ON experiments(created_at DESC);

CREATE TABLE IF NOT EXISTS interventions (
    id VARCHAR(64) PRIMARY KEY,
    experiment_id VARCHAR(64) NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    tick BIGINT NOT NULL,
    sequence INT NOT NULL DEFAULT 0,
    type VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) DEFAULT '',
    value DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration BIGINT NOT NULL DEFAULT 0,
    name VARCHAR(255) DEFAULT '',
    color VARCHAR(32) DEFAULT '',
    genome JSONB,
    params JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interventions_exp_tick ON interventions(experiment_id, tick);

CREATE TABLE IF NOT EXISTS simulation_metrics (
    id BIGSERIAL PRIMARY KEY,
    experiment_id VARCHAR(64) NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    tick BIGINT NOT NULL,
    population INT NOT NULL,
    active_colonies INT NOT NULL DEFAULT 0,
    survival_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    efficiency DOUBLE PRECISION NOT NULL DEFAULT 0,
    decision_entropy DOUBLE PRECISION NOT NULL DEFAULT 0,
    births_total INT NOT NULL DEFAULT 0,
    deaths_total INT NOT NULL DEFAULT 0,
    metrics_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_simulation_metrics_exp_tick ON simulation_metrics(experiment_id, tick);

CREATE TABLE IF NOT EXISTS colony_states (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    celestial_body VARCHAR(64) NOT NULL,
    life_form_type VARCHAR(64) NOT NULL,
    energy DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_energy DOUBLE PRECISION NOT NULL DEFAULT 0,
    entropy DOUBLE PRECISION NOT NULL DEFAULT 0,
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0,
    population INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'stable',
    weights JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS choice_logs (
    id BIGSERIAL PRIMARY KEY,
    dilemma_id VARCHAR(64) NOT NULL,
    selected_alternative VARCHAR(32) NOT NULL,
    entropy_delta DOUBLE PRECISION NOT NULL DEFAULT 0,
    energy_deltas JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_choice_logs_dilemma ON choice_logs(dilemma_id);

-- +goose Down
DROP TABLE IF EXISTS choice_logs;
DROP TABLE IF EXISTS colony_states;
DROP TABLE IF EXISTS simulation_metrics;
DROP TABLE IF EXISTS interventions;
DROP TABLE IF EXISTS experiments;

package experiments

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/engine"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/prng"
)

var (
	ErrExperimentNotFound = errors.New("experiment not found")
	ErrInvalidState       = errors.New("invalid experiment state for this operation")
)

// Experiment инкапсулирует симуляцию одного мира и его сообществ (ТЗ v2)
type Experiment struct {
	mu             sync.RWMutex
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	WorldID        string                  `json:"worldId"`
	Mode           model.Mode              `json:"mode"`
	Status         model.ExperimentStatus  `json:"status"`
	Seed           uint64                  `json:"seed"`
	Speed          int                     `json:"speed"` // 1x, 2x, 5x
	Parameters     model.Parameters        `json:"parameters"`
	Interventions  []*model.Intervention   `json:"interventions"`
	State          *engine.StepState       `json:"-"`
	Engine         *engine.Engine          `json:"-"`
	InitialSnapshot *model.StateSnapshot   `json:"initialSnapshot"`
	LatestSnapshot *model.StateSnapshot    `json:"latestSnapshot"`
	MetricsHistory []*model.MetricsSnapshot `json:"-"`
	StopChan       chan struct{}           `json:"-"`
	Log            zerolog.Logger          `json:"-"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`

	broadcastFunc func(snapshot *model.StateSnapshot, expID string)
}

// Manager управляет жизненным циклом экспериментов в памяти процесса
type Manager struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	log         zerolog.Logger
}

// NewManager создает новый менеджер экспериментов
func NewManager(log zerolog.Logger) *Manager {
	return &Manager{
		experiments: make(map[string]*Experiment),
		log:         log,
	}
}

// CreateExperiment создает новый эксперимент на выбранной планете
func (m *Manager) CreateExperiment(
	name string,
	worldID string,
	mode model.Mode,
	seed uint64,
	broadcastFunc func(*model.StateSnapshot, string),
) *Experiment {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := "exp-" + uuid.New().String()[:8]
	if name == "" {
		name = fmt.Sprintf("Experiment-%s", id)
	}
	if worldID == "" {
		worldID = "earth"
	}
	if mode == "" {
		mode = model.ModeEvolutionary
	}
	if seed == 0 {
		seed = 42
	}

	params := model.DefaultParameters()
	initSnapshot, interventions := BuildWorldScenario(worldID, mode, seed, params)

	eng := engine.NewEngine(params)
	streams := prng.NewStreams(seed)
	env := environment.NewEnvironmentModule(initSnapshot.World)

	indMap := make(map[string]*model.Individual)
	for _, ind := range initSnapshot.Individuals {
		cp := ind
		indMap[ind.ID] = &cp
	}

	colMap := make(map[string]*model.Colony)
	for _, col := range initSnapshot.Colonies {
		cp := col
		colMap[col.ID] = &cp
	}

	chMap := make(map[string]*model.Channel)
	for _, ch := range initSnapshot.Channels {
		cp := ch
		chMap[ch.ID] = &cp
	}

	stState := &engine.StepState{
		Tick:          0,
		Revision:      1,
		Mode:          mode,
		World:         initSnapshot.World,
		Env:           env,
		Individuals:   indMap,
		Colonies:      colMap,
		Channels:      chMap,
		InTransit:     make([]*model.ResourcePacket, 0),
		Signals:       make([]*model.SignalMessage, 0),
		Balance:       initSnapshot.Balance,
		Streams:       streams,
		InitialCount:  len(indMap),
		Interventions: interventions,
		NextColonyNum: len(colMap) + 1,
	}

	exp := &Experiment{
		ID:              id,
		Name:            name,
		WorldID:         worldID,
		Mode:            mode,
		Status:          model.StatusReady,
		Seed:            seed,
		Speed:           1,
		Parameters:      params,
		Interventions:   interventions,
		State:           stState,
		Engine:          eng,
		InitialSnapshot: initSnapshot,
		LatestSnapshot:  initSnapshot,
		MetricsHistory:  make([]*model.MetricsSnapshot, 0, 2000),
		StopChan:        make(chan struct{}),
		Log:             m.log.With().Str("expID", id).Logger(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		broadcastFunc:   broadcastFunc,
	}

	exp.MetricsHistory = append(exp.MetricsHistory, &initSnapshot.Metrics)
	m.experiments[id] = exp

	return exp
}

// GetExperiment возвращает эксперимент по ID
func (m *Manager) GetExperiment(id string) (*Experiment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, ok := m.experiments[id]
	if !ok {
		return nil, ErrExperimentNotFound
	}
	return exp, nil
}

// ListExperiments возвращает список всех экспериментов
func (m *Manager) ListExperiments() []*Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Experiment, 0, len(m.experiments))
	for _, exp := range m.experiments {
		list = append(list, exp)
	}
	return list
}

// SendCommand выполняет команду управления: start, pause, resume, step, setSpeed
func (exp *Experiment) SendCommand(cmd string, speed int) error {
	exp.mu.Lock()
	defer exp.mu.Unlock()

	switch cmd {
	case "start", "resume":
		if exp.Status == model.StatusRunning {
			return nil
		}
		exp.Status = model.StatusRunning
		exp.UpdatedAt = time.Now()
		go exp.runLoop()

	case "pause":
		if exp.Status != model.StatusRunning {
			return nil
		}
		exp.Status = model.StatusPaused
		exp.UpdatedAt = time.Now()

	case "step":
		if exp.Status == model.StatusRunning {
			return errors.New("cannot step while running")
		}
		snapshot, metrics, err := exp.Engine.Step(exp.State)
		if err != nil {
			exp.Status = model.StatusError
			return err
		}
		exp.LatestSnapshot = snapshot
		exp.MetricsHistory = append(exp.MetricsHistory, metrics)
		exp.UpdatedAt = time.Now()
		if exp.broadcastFunc != nil {
			exp.broadcastFunc(snapshot, exp.ID)
		}

	case "setSpeed":
		if speed < 1 {
			speed = 1
		}
		if speed > 5 {
			speed = 5
		}
		exp.Speed = speed

	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}

	return nil
}

// Replay воспроизводит симуляцию от такт 0 до targetTick и проверяет контрольную сумму (раздел 14 ТЗ v2)
func (exp *Experiment) Replay(targetTick int64) (*model.StateSnapshot, error) {
	exp.mu.Lock()
	defer exp.mu.Unlock()

	if exp.Status == model.StatusRunning {
		return nil, errors.New("cannot replay while experiment is running")
	}

	// Пересоздаем чистый движок и поток PRNG с тем же seed
	params := exp.Parameters
	initSnapshot, _ := BuildWorldScenario(exp.WorldID, exp.Mode, exp.Seed, params)
	eng := engine.NewEngine(params)
	streams := prng.NewStreams(exp.Seed)
	env := environment.NewEnvironmentModule(initSnapshot.World)

	indMap := make(map[string]*model.Individual)
	for _, ind := range initSnapshot.Individuals {
		cp := ind
		indMap[ind.ID] = &cp
	}

	colMap := make(map[string]*model.Colony)
	for _, col := range initSnapshot.Colonies {
		cp := col
		colMap[col.ID] = &cp
	}

	chMap := make(map[string]*model.Channel)
	for _, ch := range initSnapshot.Channels {
		cp := ch
		chMap[ch.ID] = &cp
	}

	stState := &engine.StepState{
		Tick:          0,
		Revision:      1,
		Mode:          exp.Mode,
		World:         initSnapshot.World,
		Env:           env,
		Individuals:   indMap,
		Colonies:      colMap,
		Channels:      chMap,
		InTransit:     make([]*model.ResourcePacket, 0),
		Signals:       make([]*model.SignalMessage, 0),
		Balance:       initSnapshot.Balance,
		Streams:       streams,
		InitialCount:  len(indMap),
		Interventions: exp.Interventions,
		NextColonyNum: len(colMap) + 1,
	}

	var latestSnap *model.StateSnapshot = initSnapshot
	var latestMetrics *model.MetricsSnapshot = &initSnapshot.Metrics
	metricsHistory := make([]*model.MetricsSnapshot, 0, targetTick+1)
	metricsHistory = append(metricsHistory, latestMetrics)

	for stState.Tick < targetTick {
		snap, m, err := eng.Step(stState)
		if err != nil {
			return nil, fmt.Errorf("replay failed at tick %d: %w", stState.Tick, err)
		}
		latestSnap = snap
		latestMetrics = m
		metricsHistory = append(metricsHistory, m)
	}

	exp.State = stState
	exp.LatestSnapshot = latestSnap
	exp.MetricsHistory = metricsHistory
	exp.UpdatedAt = time.Now()

	return latestSnap, nil
}

func (exp *Experiment) runLoop() {
	exp.Log.Info().Msg("Starting simulation loop")

	for {
		exp.mu.Lock()
		if exp.Status != model.StatusRunning {
			exp.mu.Unlock()
			break
		}

		snapshot, metrics, err := exp.Engine.Step(exp.State)
		if err != nil {
			exp.Status = model.StatusError
			exp.mu.Unlock()
			exp.Log.Error().Err(err).Msg("Simulation error")
			break
		}

		exp.LatestSnapshot = snapshot
		exp.MetricsHistory = append(exp.MetricsHistory, metrics)
		exp.UpdatedAt = time.Now()

		if metrics.Population == 0 {
			exp.Status = model.StatusCompleted
			exp.Log.Info().Msg("Simulation completed: extinction")
			exp.mu.Unlock()
			if exp.broadcastFunc != nil {
				exp.broadcastFunc(snapshot, exp.ID)
			}
			break
		}

		if snapshot.Tick >= 2000 {
			exp.Status = model.StatusCompleted
			exp.Log.Info().Msg("Simulation completed: target tick 2000 reached")
			exp.mu.Unlock()
			if exp.broadcastFunc != nil {
				exp.broadcastFunc(snapshot, exp.ID)
			}
			break
		}

		speed := exp.Speed
		broadcast := exp.broadcastFunc
		exp.mu.Unlock()

		if broadcast != nil {
			broadcast(snapshot, exp.ID)
		}

		baseDelay := 100 * time.Millisecond
		actualDelay := baseDelay / time.Duration(speed)
		time.Sleep(actualDelay)
	}
}

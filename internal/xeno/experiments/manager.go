package experiments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/engine"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/prng"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/store"
	"github.com/rs/zerolog"
)

var (
	ErrExperimentNotFound = errors.New("experiment not found")
	ErrInvalidState       = errors.New("invalid experiment state for this operation")
)

// Experiment инкапсулирует симуляцию одного мира и его сообществ (ТЗ v2)
type Experiment struct {
	previewMu         sync.Mutex
	previewExperiment *Experiment
	previewJournal    string
	mu                sync.RWMutex
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	WorldID           string                   `json:"worldId"`
	Mode              model.Mode               `json:"mode"`
	Status            model.ExperimentStatus   `json:"status"`
	Seed              uint64                   `json:"seed"`
	Speed             int                      `json:"speed"` // 1x, 2x, 5x
	Parameters        model.Parameters         `json:"parameters"`
	Interventions     []*model.Intervention    `json:"interventions"`
	State             *engine.StepState        `json:"-"`
	Engine            *engine.Engine           `json:"-"`
	InitialSnapshot   *model.StateSnapshot     `json:"initialSnapshot"`
	LatestSnapshot    *model.StateSnapshot     `json:"latestSnapshot"`
	MetricsHistory    []*model.MetricsSnapshot `json:"-"`
	StopChan          chan struct{}            `json:"-"`
	Log               zerolog.Logger           `json:"-"`
	CreatedAt         time.Time                `json:"createdAt"`
	UpdatedAt         time.Time                `json:"updatedAt"`

	loopGeneration uint64
	broadcastFunc  func(snapshot *model.StateSnapshot, expID string)
	pgRepo         *store.PostgresRepository
	redisCache     *store.RedisCache
}

// Manager управляет жизненным циклом экспериментов в памяти процесса
type Manager struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	log         zerolog.Logger
	pgRepo      *store.PostgresRepository
	redisCache  *store.RedisCache
}

// NewManager создает новый менеджер экспериментов
func NewManager(log zerolog.Logger) *Manager {
	return &Manager{
		experiments: make(map[string]*Experiment),
		log:         log,
	}
}

// SetStore binds PostgreSQL repository and Redis cache to the manager
func (m *Manager) SetStore(pg *store.PostgresRepository, rc *store.RedisCache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pgRepo = pg
	m.redisCache = rc
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
		pgRepo:          m.pgRepo,
		redisCache:      m.redisCache,
	}

	exp.MetricsHistory = append(exp.MetricsHistory, &initSnapshot.Metrics)
	initSnapshot.Mode = mode
	initSnapshot.Flow = 1
	initSnapshot.Noise = 1
	initSnapshot.Interventions = interventions
	m.experiments[id] = exp

	if m.pgRepo != nil {
		go func(p *store.PersistedExperiment) {
			_ = m.pgRepo.SaveExperiment(context.Background(), p)
		}(exp.ToPersisted())
	}
	if m.redisCache != nil {
		go func(eid string, snap *model.StateSnapshot) {
			_ = m.redisCache.CacheSnapshot(context.Background(), eid, snap)
		}(exp.ID, initSnapshot)
	}

	return exp
}

// toPersistedLocked converts in-memory Experiment into store.PersistedExperiment without locking (must hold exp.mu)
func (exp *Experiment) toPersistedLocked() *store.PersistedExperiment {
	var p store.PersistedExperiment
	p.ID = exp.ID
	p.Name = exp.Name
	p.WorldID = exp.WorldID
	p.Mode = exp.Mode
	p.Status = exp.Status
	p.Seed = exp.Seed
	p.Speed = exp.Speed
	p.Parameters = exp.Parameters
	p.Interventions = exp.Interventions
	p.InitialSnapshot = exp.InitialSnapshot
	p.LatestSnapshot = exp.LatestSnapshot
	p.MetricsHistory = exp.MetricsHistory
	p.CreatedAt = exp.CreatedAt
	p.UpdatedAt = exp.UpdatedAt
	return &p
}

// ToPersisted converts in-memory Experiment into store.PersistedExperiment
func (exp *Experiment) ToPersisted() *store.PersistedExperiment {
	exp.mu.RLock()
	defer exp.mu.RUnlock()
	return exp.toPersistedLocked()
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
		if exp.State.Tick >= 2000 {
			return errors.New("experiment completed")
		}
		if speed == 1 || speed == 2 || speed == 5 {
			exp.Speed = speed
		}
		exp.Status = model.StatusRunning
		exp.UpdatedAt = time.Now()
		exp.loopGeneration++
		if exp.redisCache != nil {
			_ = exp.redisCache.TrackActive(context.Background(), exp.ID, true)
		}
		go exp.runLoop(exp.loopGeneration)

	case "pause":
		if exp.Status != model.StatusRunning {
			return nil
		}
		exp.Status = model.StatusPaused
		exp.loopGeneration++
		exp.UpdatedAt = time.Now()
		if exp.redisCache != nil {
			_ = exp.redisCache.TrackActive(context.Background(), exp.ID, false)
		}
		if exp.pgRepo != nil {
			go func(p *store.PersistedExperiment) {
				_ = exp.pgRepo.UpdateExperimentState(context.Background(), p.ID, p.Status, p.LatestSnapshot, p.MetricsHistory, p.Speed)
			}(exp.toPersistedLocked())
		}

	case "step":
		if exp.State.Tick >= 2000 {
			return errors.New("experiment completed")
		}
		if exp.Status == model.StatusRunning {
			return errors.New("cannot step while running")
		}
		snapshot, metrics, err := exp.Engine.Step(exp.State)
		if err != nil {
			exp.Status = model.StatusError
			return err
		}
		if snapshot.Tick >= 2000 {
			exp.Status = model.StatusCompleted
		}
		snapshot.Status = exp.Status
		exp.LatestSnapshot = snapshot
		exp.MetricsHistory = append(exp.MetricsHistory, metrics)
		exp.UpdatedAt = time.Now()
		if exp.broadcastFunc != nil {
			exp.broadcastFunc(snapshot, exp.ID)
		}
		if exp.redisCache != nil {
			go func(eid string, snap *model.StateSnapshot) {
				_ = exp.redisCache.CacheSnapshot(context.Background(), eid, snap)
			}(exp.ID, snapshot)
		}
		if exp.pgRepo != nil {
			go func(p *store.PersistedExperiment, mSnap *model.MetricsSnapshot) {
				_ = exp.pgRepo.UpdateExperimentState(context.Background(), p.ID, p.Status, p.LatestSnapshot, p.MetricsHistory, p.Speed)
				_ = exp.pgRepo.SaveMetricsSnapshot(context.Background(), p.ID, mSnap)
			}(exp.toPersistedLocked(), metrics)
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

	cp := *exp.LatestSnapshot
	cp.Status = exp.Status
	exp.LatestSnapshot = &cp
	return nil
}

// Replay воспроизводит симуляцию от такт 0 до targetTick и проверяет контрольную сумму (раздел 14 ТЗ v2)
func (exp *Experiment) Replay(targetTick int64) (*model.StateSnapshot, error) {
	return exp.ReplayContext(context.Background(), targetTick, nil)
}

func (exp *Experiment) ReplayContext(ctx context.Context, targetTick int64, progress func(int64)) (*model.StateSnapshot, error) {
	if targetTick < 0 || targetTick > 2000 {
		return nil, errors.New("targetTick must be between 0 and 2000")
	}
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

	initSnapshot.Mode = exp.Mode
	initSnapshot.Flow = 1
	initSnapshot.Noise = 1
	initSnapshot.Interventions = exp.Interventions
	var latestSnap *model.StateSnapshot = initSnapshot
	var latestMetrics *model.MetricsSnapshot = &initSnapshot.Metrics
	metricsHistory := make([]*model.MetricsSnapshot, 0, targetTick+1)
	metricsHistory = append(metricsHistory, latestMetrics)

	for stState.Tick < targetTick {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		snap, m, err := eng.Step(stState)
		if err != nil {
			return nil, fmt.Errorf("replay failed at tick %d: %w", stState.Tick, err)
		}
		latestSnap = snap
		latestMetrics = m
		metricsHistory = append(metricsHistory, m)
		if progress != nil {
			progress(stState.Tick)
		}
	}

	exp.Status = model.StatusPaused
	if targetTick >= 2000 {
		exp.Status = model.StatusCompleted
	}
	latestSnap.Status = exp.Status
	latestSnap.Mode = stState.Mode
	exp.State = stState
	exp.LatestSnapshot = latestSnap
	exp.MetricsHistory = metricsHistory
	exp.UpdatedAt = time.Now()

	return latestSnap, nil
}

func (exp *Experiment) runLoop(generation uint64) {
	exp.Log.Info().Msg("Starting simulation loop")

	for {
		exp.mu.Lock()
		if exp.Status != model.StatusRunning || generation != exp.loopGeneration {
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

		// Real-time Redis cache update
		if exp.redisCache != nil {
			go func(eid string, snap *model.StateSnapshot) {
				_ = exp.redisCache.CacheSnapshot(context.Background(), eid, snap)
			}(exp.ID, snapshot)
		}

		// Periodic PostgreSQL persistence
		if snapshot.Tick%5 == 0 || metrics.Population == 0 || snapshot.Tick >= 2000 {
			if exp.pgRepo != nil {
				go func(p *store.PersistedExperiment, mSnap *model.MetricsSnapshot) {
					_ = exp.pgRepo.UpdateExperimentState(context.Background(), p.ID, p.Status, p.LatestSnapshot, p.MetricsHistory, p.Speed)
					_ = exp.pgRepo.SaveMetricsSnapshot(context.Background(), p.ID, mSnap)
				}(exp.toPersistedLocked(), metrics)
			}
		}

		if metrics.Population == 0 {
			exp.Status = model.StatusCompleted
			snapshot.Status = exp.Status
			exp.Log.Info().Msg("Simulation completed: extinction")
			exp.mu.Unlock()
			if exp.broadcastFunc != nil {
				exp.broadcastFunc(snapshot, exp.ID)
			}
			break
		}

		if snapshot.Tick >= 2000 {
			exp.Status = model.StatusCompleted
			snapshot.Status = exp.Status
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

	// Cleanup active state on loop exit
	if exp.redisCache != nil {
		_ = exp.redisCache.TrackActive(context.Background(), exp.ID, false)
	}
	if exp.pgRepo != nil {
		_ = exp.pgRepo.SaveExperiment(context.Background(), exp.ToPersisted())
	}
}

// View returns a detached consistent copy for all transports and exporters.
func (exp *Experiment) View() *Experiment {
	exp.mu.RLock()
	defer exp.mu.RUnlock()
	b, _ := json.Marshal(exp)
	var cp Experiment
	_ = json.Unmarshal(b, &cp)
	cp.MetricsHistory = make([]*model.MetricsSnapshot, len(exp.MetricsHistory))
	for i, m := range exp.MetricsHistory {
		v := *m
		cp.MetricsHistory[i] = &v
	}
	if cp.LatestSnapshot != nil {
		cp.LatestSnapshot.Status = cp.Status
	}
	return &cp
}
func (exp *Experiment) AddIntervention(it model.Intervention) (*model.Intervention, error) {
	exp.mu.Lock()
	defer exp.mu.Unlock()
	if exp.State.Tick >= 2000 {
		return nil, errors.New("experiment reached 2000 ticks; create a new experiment")
	}
	if math.IsNaN(it.Value) || math.IsInf(it.Value, 0) {
		return nil, errors.New("invalid value")
	}
	for _, v := range it.Params {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("invalid parameter")
		}
	}
	switch it.Type {
	case "set_flow", "set_noise":
		if it.Value < 0 || it.Value > 10 {
			return nil, errors.New("value must be 0..10")
		}
	case "impulse", "perturbation", "depletion":
		if it.Duration == 0 {
			it.Duration = 60
		}
		if it.Duration < 1 || it.Duration > 1000 {
			return nil, errors.New("duration must be 1..1000")
		}
	case "toggle_mutations":
		if it.Value != 0 && it.Value != 1 {
			return nil, errors.New("mutations must be 0 or 1")
		}
	case "set_mode":
		if !ValidMode(model.Mode(it.TargetID)) {
			return nil, errors.New("invalid mode")
		}
	case "set_channel":
		if exp.State.Channels[it.TargetID] == nil {
			return nil, errors.New("unknown channel")
		}
		for k, v := range it.Params {
			if (k == "power" && (v < 0 || v > 20)) || (k == "loss" && (v < 0 || v > 0.9)) || (k == "delay" && (v < 1 || v > 100 || v != math.Trunc(v))) {
				return nil, errors.New("invalid channel parameter")
			}
		}
	case "add_inoculum":
		n := it.Params["count"]
		if n == 0 {
			n = 6
		}
		if n < 1 || n > 24 || n != math.Trunc(n) {
			return nil, errors.New("count must be 1..24")
		}
		if len(exp.State.Individuals)+int(n) > exp.Parameters.MaxPopulation || len(exp.State.Colonies) >= exp.State.World.Model.MaxColonies {
			return nil, errors.New("population or colony limit reached")
		}
		if math.Abs(it.Params["lat"]) > 90 || math.Abs(it.Params["lng"]) > 180 || it.Value < 0 || it.Value > exp.Parameters.EMax || it.Params["biomass"] < 0 || it.Params["biomass"] > 20 || it.Params["spread"] < 0 || it.Params["spread"] > 15 || it.Params["power"] < 0 || it.Params["power"] > 20 {
			return nil, errors.New("invalid colony parameters")
		}
		if len([]rune(it.Name)) > 80 {
			return nil, errors.New("name too long")
		}
		if it.Color != "" && !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(it.Color) {
			return nil, errors.New("invalid color")
		}
		if it.Genome != nil {
			g := it.Genome
			sum := g.WeightEnergy + g.WeightDeficit + g.WeightRelief + g.WeightReproduction + g.WeightCost
			for _, v := range []float64{g.WeightEnergy, g.WeightDeficit, g.WeightRelief, g.WeightReproduction, g.WeightCost, g.Lambda, g.HThreshold} {
				if v < 0 || v > 1 || math.IsNaN(v) {
					return nil, errors.New("invalid genome")
				}
			}
			if sum <= 0 {
				return nil, errors.New("empty genome")
			}
			g.WeightEnergy /= sum
			g.WeightDeficit /= sum
			g.WeightRelief /= sum
			g.WeightReproduction /= sum
			g.WeightCost /= sum
		}
	default:
		return nil, errors.New("unknown intervention")
	}
	it.ID = "event-" + uuid.NewString()
	it.Tick = exp.State.Tick + 1
	it.Sequence = len(exp.Interventions) + 1
	exp.Interventions = append(exp.Interventions, &it)
	exp.State.Interventions = exp.Interventions

	if exp.pgRepo != nil {
		go func(eid string, itCopy model.Intervention, p *store.PersistedExperiment) {
			_ = exp.pgRepo.SaveIntervention(context.Background(), eid, &itCopy)
			_ = exp.pgRepo.SaveExperiment(context.Background(), p)
		}(exp.ID, it, exp.toPersistedLocked())
	}
	if exp.redisCache != nil {
		go func(eid string, snap *model.StateSnapshot) {
			_ = exp.redisCache.CacheSnapshot(context.Background(), eid, snap)
		}(exp.ID, exp.LatestSnapshot)
	}

	return &it, nil
}
func ValidMode(mode model.Mode) bool {
	return mode == model.ModeReactive || mode == model.ModeAdaptive || mode == model.ModeEvolutionary
}

// Preview is read-only: scrubbing a recording never rewinds the source experiment.
func (exp *Experiment) Preview(tick int64) (*model.StateSnapshot, error) {
	exp.previewMu.Lock()
	defer exp.previewMu.Unlock()
	v := exp.View()
	if tick < 0 || tick > v.LatestSnapshot.Tick {
		return nil, errors.New("tick outside recording")
	}
	journal, _ := json.Marshal(v.Interventions)
	tmp := exp.previewExperiment
	if tmp == nil || string(journal) != exp.previewJournal || tick < tmp.State.Tick {
		tmp = NewManager(zerolog.Nop()).CreateExperiment(v.Name, v.WorldID, v.Mode, v.Seed, nil)
		tmp.Parameters = v.Parameters
		tmp.Interventions = v.Interventions
		if _, err := tmp.Replay(0); err != nil {
			return nil, err
		}
		exp.previewExperiment = tmp
		exp.previewJournal = string(journal)
	}
	for tmp.State.Tick < tick {
		if err := tmp.SendCommand("step", 1); err != nil {
			return nil, err
		}
	}
	return tmp.View().LatestSnapshot, nil
}
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp := m.experiments[id]; exp != nil {
		exp.SendCommand("pause", 1)
		delete(m.experiments, id)
	}
	if m.pgRepo != nil {
		go func() {
			_ = m.pgRepo.DeleteExperiment(context.Background(), id)
		}()
	}
	if m.redisCache != nil {
		go func() {
			_ = m.redisCache.InvalidateExperiment(context.Background(), id)
		}()
	}
}

// RestoreFromDB restores experiments from PostgreSQL into the in-memory manager
func (m *Manager) RestoreFromDB(ctx context.Context, broadcast func(*model.StateSnapshot, string)) error {
	if m.pgRepo == nil {
		return nil
	}

	savedList, err := m.pgRepo.ListExperiments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list experiments from PostgreSQL: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, saved := range savedList {
		if saved == nil || saved.ID == "" {
			continue
		}
		if _, exists := m.experiments[saved.ID]; exists {
			continue
		}

		params := saved.Parameters
		initSnapshot, interventions := BuildWorldScenario(saved.WorldID, saved.Mode, saved.Seed, params)
		eng := engine.NewEngine(params)
		streams := prng.NewStreams(saved.Seed)
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

		allInterventions := saved.Interventions
		if len(allInterventions) == 0 {
			allInterventions = interventions
		}

		stState := &engine.StepState{
			Tick:          0,
			Revision:      1,
			Mode:          saved.Mode,
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
			Interventions: allInterventions,
			NextColonyNum: len(colMap) + 1,
		}

		exp := &Experiment{
			ID:              saved.ID,
			Name:            saved.Name,
			WorldID:         saved.WorldID,
			Mode:            saved.Mode,
			Status:          saved.Status,
			Seed:            saved.Seed,
			Speed:           saved.Speed,
			Parameters:      saved.Parameters,
			Interventions:   allInterventions,
			State:           stState,
			Engine:          eng,
			InitialSnapshot: saved.InitialSnapshot,
			LatestSnapshot:  saved.LatestSnapshot,
			MetricsHistory:  saved.MetricsHistory,
			StopChan:        make(chan struct{}),
			Log:             m.log.With().Str("expID", saved.ID).Logger(),
			CreatedAt:       saved.CreatedAt,
			UpdatedAt:       saved.UpdatedAt,
			broadcastFunc:   broadcast,
			pgRepo:          m.pgRepo,
			redisCache:      m.redisCache,
		}

		if exp.InitialSnapshot == nil {
			exp.InitialSnapshot = initSnapshot
		}
		if exp.LatestSnapshot == nil {
			exp.LatestSnapshot = initSnapshot
		}
		if len(exp.MetricsHistory) == 0 {
			exp.MetricsHistory = []*model.MetricsSnapshot{&exp.InitialSnapshot.Metrics}
		}

		// If the experiment was previously stepped, replay engine state
		if exp.LatestSnapshot.Tick > 0 {
			for stState.Tick < exp.LatestSnapshot.Tick {
				snap, mSnap, err := eng.Step(stState)
				if err != nil {
					break
				}
				exp.LatestSnapshot = snap
				_ = mSnap
			}
		}

		// Ensure loaded experiments start in Paused status if they were running
		if exp.Status == model.StatusRunning {
			exp.Status = model.StatusPaused
		}
		exp.LatestSnapshot.Status = exp.Status

		m.experiments[exp.ID] = exp

		if m.redisCache != nil {
			go func(eid string, snap *model.StateSnapshot, mHist []*model.MetricsSnapshot) {
				_ = m.redisCache.CacheSnapshot(context.Background(), eid, snap)
				_ = m.redisCache.CacheMetrics(context.Background(), eid, mHist)
			}(exp.ID, exp.LatestSnapshot, exp.MetricsHistory)
		}
	}

	m.log.Info().Int("count", len(savedList)).Msg("Experiments successfully restored from PostgreSQL")
	return nil
}

// StartDBSync launches background periodic sync of active experiments to PostgreSQL
func (m *Manager) StartDBSync(interval time.Duration) {
	if m.pgRepo == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.mu.RLock()
			for _, exp := range m.experiments {
				if exp != nil && (exp.Status == model.StatusRunning || exp.Status == model.StatusPaused) {
					p := exp.ToPersisted()
					go func(pers *store.PersistedExperiment) {
						_ = m.pgRepo.UpdateExperimentState(context.Background(), pers.ID, pers.Status, pers.LatestSnapshot, pers.MetricsHistory, pers.Speed)
					}(p)
				}
			}
			m.mu.RUnlock()
		}
	}()
}

// Adopt publishes a fully restored experiment only after its checksum has passed.
func (m *Manager) Adopt(exp *Experiment, broadcast func(*model.StateSnapshot, string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.experiments) >= 100 {
		return errors.New("experiment limit reached")
	}
	exp.broadcastFunc = broadcast
	exp.pgRepo = m.pgRepo
	exp.redisCache = m.redisCache
	m.experiments[exp.ID] = exp

	if m.pgRepo != nil {
		go func(p *store.PersistedExperiment) {
			_ = m.pgRepo.SaveExperiment(context.Background(), p)
		}(exp.ToPersisted())
	}
	if m.redisCache != nil && exp.LatestSnapshot != nil {
		go func(eid string, snap *model.StateSnapshot) {
			_ = m.redisCache.CacheSnapshot(context.Background(), eid, snap)
		}(exp.ID, exp.LatestSnapshot)
	}

	return nil
}

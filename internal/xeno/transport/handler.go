package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/export"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/store"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
	"github.com/rs/zerolog"
)

type Handler struct {
	importsMu   sync.Mutex
	imports     map[string]*importJob
	importSlots chan struct{}
	manager     *experiments.Manager
	log         zerolog.Logger
	wsMu        sync.RWMutex
	wsRooms     map[string]map[*websocket.Conn]bool // expID -> conns
	pgRepo      *store.PostgresRepository
	redisCache  *store.RedisCache
}

func NewHandler(manager *experiments.Manager, log zerolog.Logger) *Handler {
	return &Handler{
		manager:     manager,
		log:         log,
		wsRooms:     make(map[string]map[*websocket.Conn]bool),
		imports:     make(map[string]*importJob),
		importSlots: make(chan struct{}, 2),
	}
}

// SetStore binds PostgreSQL repository and Redis cache to the transport handler
func (h *Handler) SetStore(pg *store.PostgresRepository, rc *store.RedisCache) {
	h.pgRepo = pg
	h.redisCache = rc
}

// BroadcastSnapshot отправляет снимок состояния подписчикам WebSocket
func (h *Handler) BroadcastSnapshot(snapshot *model.StateSnapshot, expID string) {
	h.wsMu.RLock()
	clients, ok := h.wsRooms[expID]
	if !ok || len(clients) == 0 {
		h.wsMu.RUnlock()
		return
	}

	payload := fiber.Map{
		"type":         "snapshot",
		"experimentId": expID,
		"worldId":      snapshot.World.ID,
		"tick":         snapshot.Tick,
		"revision":     snapshot.Revision,
		"payload":      snapshot,
	}
	data, err := json.Marshal(payload)
	h.wsMu.RUnlock()

	if err != nil {
		return
	}

	h.wsMu.Lock()
	defer h.wsMu.Unlock()
	for conn := range clients {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}

	// Real-time broadcast over Redis Pub/Sub
	if h.redisCache != nil {
		go func() {
			_ = h.redisCache.PublishSnapshot(context.Background(), expID, data)
			_ = h.redisCache.CacheSnapshot(context.Background(), expID, snapshot)
		}()
	}
}

// GetWorlds возвращает каталог планет со справочными параметрами NASA (раздел 5 ТЗ v2)
func (h *Handler) GetWorlds(c *fiber.Ctx) error {
	if h.redisCache != nil {
		if cached, err := h.redisCache.GetCachedWorlds(c.Context()); err == nil && len(cached) > 0 {
			return response.OK(c, cached)
		}
	}

	worlds := model.GetPresetWorlds()
	if h.redisCache != nil {
		go func() {
			_ = h.redisCache.CacheWorlds(context.Background(), worlds)
		}()
	}
	return response.OK(c, worlds)
}

// CreateExperimentRequest — тело запроса на создание эксперимента
type CreateExperimentRequest struct {
	Name    string     `json:"name"`
	WorldID string     `json:"worldId"`
	Mode    model.Mode `json:"mode"`
	Seed    uint64     `json:"seed"`
}

// CreateExperiment создает эксперимент на выбранной планете
func (h *Handler) CreateExperiment(c *fiber.Ctx) error {
	var req CreateExperimentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.WorldID == "" {
		req.WorldID = "earth"
	}
	if req.Mode == "" {
		req.Mode = model.ModeEvolutionary
	}
	if req.Seed == 0 {
		req.Seed = 42
	}

	if _, ok := model.FindWorldByID(req.WorldID); !ok {
		return response.BadRequest(c, "Unknown world")
	}
	if !experiments.ValidMode(req.Mode) {
		return response.BadRequest(c, "Unknown mode")
	}
	if len(h.manager.ListExperiments()) >= 100 {
		return response.BadRequest(c, "Experiment limit reached; remove unused experiments")
	}
	exp := h.manager.CreateExperiment(req.Name, req.WorldID, req.Mode, req.Seed, h.BroadcastSnapshot)
	return response.Created(c, exp.View())
}

// ListExperiments возвращает список всех экспериментов
func (h *Handler) ListExperiments(c *fiber.Ctx) error {
	list := h.manager.ListExperiments()
	views := make([]*experiments.Experiment, 0, len(list))
	for _, exp := range list {
		views = append(views, exp.View())
	}
	return response.OK(c, views)
}

// GetExperiment возвращает детали эксперимента
func (h *Handler) GetExperiment(c *fiber.Ctx) error {
	id := c.Params("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	return response.OK(c, exp.View())
}

// CommandRequest — запрос на выполнение команды
type CommandRequest struct {
	CommandId        string `json:"commandId"`
	Command          string `json:"command"` // start, pause, resume, step, setSpeed
	Speed            int    `json:"speed,omitempty"`
	ExpectedRevision int64  `json:"expectedRevision,omitempty"`
}

// ExecuteCommand исполняет команду управления симуляцией
func (h *Handler) ExecuteCommand(c *fiber.Ctx) error {
	id := c.Params("id")
	var req CommandRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	view := exp.View()
	if req.ExpectedRevision > 0 && view.LatestSnapshot.Revision != req.ExpectedRevision {
		return response.BadRequest(c, fmt.Sprintf("revision mismatch: expected %d, got %d",
			req.ExpectedRevision, view.LatestSnapshot.Revision))
	}

	if err := exp.SendCommand(req.Command, req.Speed); err != nil {
		return response.BadRequest(c, err.Error())
	}

	exp = exp.View()
	return response.OK(c, fiber.Map{
		"experimentId": exp.ID,
		"status":       exp.Status,
		"revision":     exp.LatestSnapshot.Revision,
		"tick":         exp.LatestSnapshot.Tick,
	})
}

// AddIntervention планирует внешнее вмешательство исследователя
func (h *Handler) AddIntervention(c *fiber.Ctx) error {
	id := c.Params("id")
	var it model.Intervention
	if err := c.BodyParser(&it); err != nil {
		return response.BadRequest(c, "Invalid intervention body")
	}

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	accepted, err := exp.AddIntervention(it)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, accepted)
}

// GetStateSnapshot возвращает снимок состояния мира и чексумму SHA-256
func (h *Handler) GetStateSnapshot(c *fiber.Ctx) error {
	id := c.Params("id")
	if h.redisCache != nil {
		if cached, err := h.redisCache.GetCachedSnapshot(c.Context(), id); err == nil && cached != nil {
			return response.OK(c, cached)
		}
	}
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	snap := exp.View().LatestSnapshot
	if h.redisCache != nil && snap != nil {
		go func() {
			_ = h.redisCache.CacheSnapshot(context.Background(), id, snap)
		}()
	}
	return response.OK(c, snap)
}

// GetColony возвращает детальное состояние колонии и список её особей
func (h *Handler) GetColony(c *fiber.Ctx) error {
	id := c.Params("id")
	colonyID := c.Params("colonyId")

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	exp = exp.View()
	var targetColony *model.Colony
	for _, col := range exp.LatestSnapshot.Colonies {
		if col.ID == colonyID {
			targetColony = &col
			break
		}
	}
	if targetColony == nil {
		return response.NotFound(c, "Colony not found")
	}

	// Собираем особей данной колонии
	colonyIndividuals := make([]model.Individual, 0)
	for _, ind := range exp.View().LatestSnapshot.Individuals {
		if ind.ColonyID == colonyID {
			colonyIndividuals = append(colonyIndividuals, ind)
		}
	}

	return response.OK(c, fiber.Map{
		"colony":      targetColony,
		"individuals": colonyIndividuals,
	})
}

// GetIndividual возвращает полную трассировку особи
func (h *Handler) GetIndividual(c *fiber.Ctx) error {
	id := c.Params("id")
	indID := c.Params("individualId")

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	for _, ind := range exp.View().LatestSnapshot.Individuals {
		if ind.ID == indID {
			return response.OK(c, ind)
		}
	}

	return response.NotFound(c, "Individual not found")
}

// GetMetrics возвращает историю метрик
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	id := c.Params("id")
	fromTick := c.QueryInt("from", -1)
	toTick := c.QueryInt("to", -1)

	// If default full history is requested, check Redis cache
	if fromTick == -1 && toTick == -1 && h.redisCache != nil {
		if cached, err := h.redisCache.GetCachedMetrics(c.Context(), id); err == nil && len(cached) > 0 {
			return response.OK(c, cached)
		}
	}

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	history := exp.View().MetricsHistory
	if fromTick == -1 && toTick == -1 {
		if h.redisCache != nil {
			go func() {
				_ = h.redisCache.CacheMetrics(context.Background(), id, history)
			}()
		}
		return response.OK(c, history)
	}

	minT := int64(fromTick)
	if minT < 0 {
		minT = 0
	}
	maxT := int64(toTick)
	if maxT < 0 {
		maxT = 100000
	}

	filtered := make([]*model.MetricsSnapshot, 0)
	for _, m := range history {
		if m.Tick >= minT && m.Tick <= maxT {
			filtered = append(filtered, m)
		}
	}

	return response.OK(c, filtered)
}

// ExportExperiment экспортирует результаты прогона в JSON или CSV
func (h *Handler) ExportExperiment(c *fiber.Ctx) error {
	id := c.Params("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	format := c.Query("format", "json")
	if format == "csv" {
		data, err := export.ExportCSV(exp)
		if err != nil {
			return response.InternalError(c, err.Error())
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-metrics.csv", exp.ID))
		return c.Send(data)
	}

	data, err := export.ExportJSON(exp)
	if err != nil {
		return response.InternalError(c, err.Error())
	}
	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-export.json", exp.ID))
	return c.Send(data)
}

// ReplayRequest — запрос на воспроизведение опыта
type ReplayRequest struct {
	TargetTick int64 `json:"targetTick"`
}

// ReplayExperiment воспроизводит симуляцию от такт 0 до targetTick
func (h *Handler) ReplayExperiment(c *fiber.Ctx) error {
	id := c.Params("id")
	var req ReplayRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid replay request")
	}

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	snap, err := exp.Replay(req.TargetTick)
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OK(c, snap)
}

// UpgradeWS проверяет запрос на WebSocket upgrade
func (h *Handler) UpgradeWS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

// HandleWebSocketStream обрабатывает подключение к WebSocket стриму эксперимента
func (h *Handler) HandleWebSocketStream(c *websocket.Conn) {
	expID := c.Params("id")

	h.wsMu.Lock()
	if _, ok := h.wsRooms[expID]; !ok {
		h.wsRooms[expID] = make(map[*websocket.Conn]bool)
	}
	h.wsRooms[expID][c] = true
	h.wsMu.Unlock()

	defer func() {
		h.wsMu.Lock()
		delete(h.wsRooms[expID], c)
		if len(h.wsRooms[expID]) == 0 {
			delete(h.wsRooms, expID)
		}
		h.wsMu.Unlock()
		c.Close()
	}()

	// Отключаем таймауты чтения/записи для постоянного WebSocket-стрима
	_ = c.SetReadDeadline(time.Time{})
	_ = c.SetWriteDeadline(time.Time{})

	// Отправляем начальный снимок
	if exp, err := h.manager.GetExperiment(expID); err == nil {
		exp = exp.View()
		payload := fiber.Map{
			"type":         "snapshot",
			"experimentId": expID,
			"worldId":      exp.LatestSnapshot.World.ID,
			"tick":         exp.LatestSnapshot.Tick,
			"revision":     exp.LatestSnapshot.Revision,
			"payload":      exp.LatestSnapshot,
		}
		if b, err := json.Marshal(payload); err == nil {
			h.wsMu.Lock()
			_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
			_ = c.WriteMessage(websocket.TextMessage, b)
			h.wsMu.Unlock()
		}
	}

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}

		var cmdReq CommandRequest
		if err := json.Unmarshal(msg, &cmdReq); err == nil && cmdReq.Command != "" {
			if exp, err := h.manager.GetExperiment(expID); err == nil {
				exp.SendCommand(cmdReq.Command, cmdReq.Speed)
			}
		}
	}
}

func (h *Handler) Preview(c *fiber.Ctx) error {
	exp, err := h.manager.GetExperiment(c.Params("id"))
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	tick, err := strconv.ParseInt(c.Query("tick", "0"), 10, 64)
	if err != nil {
		return response.BadRequest(c, "Invalid tick")
	}
	snap, err := exp.Preview(tick)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, snap)
}
func (h *Handler) DeleteExperiment(c *fiber.Ctx) error {
	h.manager.Remove(c.Params("id"))
	return response.OK(c, fiber.Map{"deleted": true})
}

package transport

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/experiments"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/export"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/response"
)

type Handler struct {
	manager *experiments.Manager
	log     zerolog.Logger
	wsMu    sync.RWMutex
	wsRooms map[string]map[*websocket.Conn]bool // expID -> conns
}

func NewHandler(manager *experiments.Manager, log zerolog.Logger) *Handler {
	return &Handler{
		manager: manager,
		log:     log,
		wsRooms: make(map[string]map[*websocket.Conn]bool),
	}
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
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}
}

// GetWorlds возвращает каталог планет со справочными параметрами NASA (раздел 5 ТЗ v2)
func (h *Handler) GetWorlds(c *fiber.Ctx) error {
	worlds := model.GetPresetWorlds()
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
		req.WorldID = "earth"
		req.Mode = model.ModeEvolutionary
		req.Seed = 42
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

	exp := h.manager.CreateExperiment(req.Name, req.WorldID, req.Mode, req.Seed, h.BroadcastSnapshot)
	return response.Created(c, exp)
}

// ListExperiments возвращает список всех экспериментов
func (h *Handler) ListExperiments(c *fiber.Ctx) error {
	list := h.manager.ListExperiments()
	return response.OK(c, list)
}

// GetExperiment возвращает детали эксперимента
func (h *Handler) GetExperiment(c *fiber.Ctx) error {
	id := c.Params("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	return response.OK(c, exp)
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

	if req.ExpectedRevision > 0 && exp.LatestSnapshot.Revision != req.ExpectedRevision {
		return response.BadRequest(c, fmt.Sprintf("revision mismatch: expected %d, got %d",
			req.ExpectedRevision, exp.LatestSnapshot.Revision))
	}

	if err := exp.SendCommand(req.Command, req.Speed); err != nil {
		return response.BadRequest(c, err.Error())
	}

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

	exp.Interventions = append(exp.Interventions, &it)
	return response.Created(c, it)
}

// GetStateSnapshot возвращает снимок состояния мира и чексумму SHA-256
func (h *Handler) GetStateSnapshot(c *fiber.Ctx) error {
	id := c.Params("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}
	return response.OK(c, exp.LatestSnapshot)
}

// GetColony возвращает детальное состояние колонии и список её особей
func (h *Handler) GetColony(c *fiber.Ctx) error {
	id := c.Params("id")
	colonyID := c.Params("colonyId")

	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

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
	for _, ind := range exp.LatestSnapshot.Individuals {
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

	for _, ind := range exp.LatestSnapshot.Individuals {
		if ind.ID == indID {
			return response.OK(c, ind)
		}
	}

	return response.NotFound(c, "Individual not found")
}

// GetMetrics возвращает историю метрик
func (h *Handler) GetMetrics(c *fiber.Ctx) error {
	id := c.Params("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		return response.NotFound(c, "Experiment not found")
	}

	fromTick := c.QueryInt("from", 0)
	toTick := c.QueryInt("to", 100000)

	filtered := make([]*model.MetricsSnapshot, 0)
	for _, m := range exp.MetricsHistory {
		if m.Tick >= int64(fromTick) && m.Tick <= int64(toTick) {
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

// ImportExperiment импортирует ранее сохраненный эксперимент
func (h *Handler) ImportExperiment(c *fiber.Ctx) error {
	body := c.Body()
	bundle, err := export.ImportJSON(body)
	if err != nil {
		return response.BadRequest(c, fmt.Sprintf("Import error: %v", err))
	}

	return response.Created(c, bundle)
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

	// Отправляем начальный снимок
	if exp, err := h.manager.GetExperiment(expID); err == nil {
		payload := fiber.Map{
			"type":         "snapshot",
			"experimentId": expID,
			"worldId":      exp.LatestSnapshot.World.ID,
			"tick":         exp.LatestSnapshot.Tick,
			"revision":     exp.LatestSnapshot.Revision,
			"payload":      exp.LatestSnapshot,
		}
		if b, err := json.Marshal(payload); err == nil {
			c.WriteMessage(websocket.TextMessage, b)
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

package model

// Mode — режим принятия решений
type Mode string

const (
	ModeReactive     Mode = "reactive"     // Фиксированный автомат без учета динамической памяти
	ModeAdaptive     Mode = "adaptive"     // Оценка альтернатив с динамической памятью притока
	ModeEvolutionary Mode = "evolutionary" // Наследуемые различия и мутации генома
)

// ExperimentStatus — статус жизненного цикла эксперимента
type ExperimentStatus string

const (
	StatusDraft     ExperimentStatus = "draft"
	StatusReady     ExperimentStatus = "ready"
	StatusRunning   ExperimentStatus = "running"
	StatusPaused    ExperimentStatus = "paused"
	StatusCompleted ExperimentStatus = "completed"
	StatusError     ExperimentStatus = "error"
)

// ActionType — 4 класса допустимых действий особи (раздел 7.3 ТЗ v2)
type ActionType string

const (
	ActionStore    ActionType = "STORE"    // Накопление / сохранение ресурса
	ActionTransfer ActionType = "TRANSFER" // Передача сигнала/ресурса по каналу связи
	ActionGrow     ActionType = "GROW"     // Развитие структуры (конверсия энергии в биомассу/структуру)
	ActionDivide   ActionType = "DIVIDE"   // Воспроизводство / деление особи
)

// Genome — наследуемые параметры целевой функции особи (раздел 7.3 ТЗ v2)
type Genome struct {
	WeightEnergy       float64 `json:"wE"`         // Вес ожидаемого запаса (wE)
	WeightDeficit      float64 `json:"wD"`         // Штраф за ожидаемый дефицит (wD)
	WeightRelief       float64 `json:"wC"`         // Вес помощи соседям (wC)
	WeightReproduction float64 `json:"wR"`         // Вес результата размножения (wR)
	WeightCost         float64 `json:"wCost"`      // Штраф за необратимые затраты (wCost)
	Lambda             float64 `json:"lambda"`     // Коэффициент памяти M_next = lambda*M + (1-lambda)*netInflow
	HThreshold         float64 `json:"hThreshold"` // Порог улучшения h для смены действия
}

// Individual — особь: гипотетическая небиологическая структура (раздел 4.2 ТЗ v2)
type Individual struct {
	ID               string         `json:"id"`
	WorldID          string         `json:"worldId"`
	ColonyID         string         `json:"colonyId"`
	ParentID         *string        `json:"parentId,omitempty"`
	Lat              float64        `json:"lat"`                 // Широта на сфере (-90..+90)
	Lng              float64        `json:"lng"`                 // Долгота на сфере (-180..+180)
	Energy           float64        `json:"energy"`              // Аккумулированная энергия
	Biomass          float64        `json:"biomass"`             // Структурный ресурс
	Memory           float64        `json:"memory"`              // Динамическая память притока M
	Genome           Genome         `json:"genome"`              // Наследуемые параметры
	Alive            bool           `json:"alive"`               // Статус жизнеспособности
	Age              int64          `json:"age"`                 // Возраст в тактах
	Generation       int            `json:"generation"`          // Номер поколения
	StarvationTicks  int            `json:"starvationTicks"`     // Число тактов в режиме дефицита
	BirthTick        int64          `json:"birthTick"`           // Такт появления
	DeathTick        *int64         `json:"deathTick,omitempty"` // Такт гибели
	DeathReason      string         `json:"deathReason,omitempty"`
	LastDecision     *DecisionTrace `json:"lastDecision,omitempty"`
	LastDivisionTick int64          `json:"lastDivisionTick"`
}

// ColonyMetrics — агрегированные показатели сообщества
type ColonyMetrics struct {
	Population           int                `json:"population"`
	TotalEnergy          float64            `json:"totalEnergy"`
	TotalBiomass         float64            `json:"totalBiomass"`
	MortalityRate        float64            `json:"mortalityRate"`
	CommunicationLevel   float64            `json:"communicationLevel"`
	DecisionDistribution map[ActionType]int `json:"decisionDistribution"`
}

// Colony — сообщество взаимодействующих особей на планете (раздел 4.3 ТЗ v2)
type Colony struct {
	ID             string        `json:"id"`
	WorldID        string        `json:"worldId"`
	ParentColonyID *string       `json:"parentColonyId,omitempty"`
	Name           string        `json:"name"`
	Color          string        `json:"color"`         // Цвет в HEX (#3b82f6)
	FormedAtTick   int64         `json:"formedAtTick"`  // Такт образования
	IndividualIDs  []string      `json:"individualIds"` // Список живых особей
	Metrics        ColonyMetrics `json:"metrics"`       // Агрегаты участников
}

// Channel — физическая связь между двумя особями (раздел 4.4 ТЗ v2)
type Channel struct {
	ID          string  `json:"id"`
	FromID      string  `json:"fromId"`
	ToID        string  `json:"toId"`
	Distance    float64 `json:"distance"`    // Ортодромное расстояние на сфере
	Conductance float64 `json:"conductance"` // Проводимость G
	MaxPower    float64 `json:"maxPower"`    // Предельная мощность Pmax
	Loss        float64 `json:"loss"`        // Доля потерь (0.0..0.9)
	DelayTicks  int     `json:"delayTicks"`  // Задержка доставки (такты)
	Enabled     bool    `json:"enabled"`     // Доступность канала
}

// SignalMessage — наблюдение/сигнал между особями (раздел 4.5 ТЗ v2)
type SignalMessage struct {
	ID           string  `json:"id"`
	SenderID     string  `json:"senderId"`
	ReceiverID   string  `json:"receiverId"`
	NormalizedE  float64 `json:"normalizedE"` // Нормированный запас отправителя
	Value        float64 `json:"value"`       // Физическая величина (потенциал / давление / частота)
	IsStarving   bool    `json:"isStarving"`  // Сигнал бедствия при голодании
	EmittedTick  int64   `json:"emittedTick"`
	DeliveryTick int64   `json:"deliveryTick"`
}

// ResourcePacket — физический пакет энергии в пути по каналу (раздел 4.5 ТЗ v2)
type ResourcePacket struct {
	ID           string  `json:"id"`
	ChannelID    string  `json:"channelId"`
	SenderID     string  `json:"senderId"`
	ReceiverID   string  `json:"receiverId"`
	EmittedTick  int64   `json:"emittedTick"`
	SentEnergy   float64 `json:"sentEnergy"` // Отправленная энергия
	NetEnergy    float64 `json:"netEnergy"`  // Полезная энергия к доставке
	LossEnergy   float64 `json:"lossEnergy"` // Рассеяно в канале
	DeliveryTick int64   `json:"deliveryTick"`
}

// DecisionTrace — полная трассировка выбора действия особи (раздел 7.3 ТЗ v2)
type DecisionTrace struct {
	IndividualID   string                 `json:"individualId"`
	Mode           Mode                   `json:"mode"`
	Tick           int64                  `json:"tick"`
	SelectedAction ActionType             `json:"selectedAction"`
	SelectedTarget string                 `json:"selectedTarget,omitempty"` // ID соседа при TRANSFER
	Scores         map[ActionType]float64 `json:"scores"`                   // score(a) по 4 классам
	ChosenScore    float64                `json:"chosenScore"`
	Reasoning      string                 `json:"reasoning,omitempty"`
}

// Intervention — внешнее воздействие исследователя (раздел 8 ТЗ v2)
type Intervention struct {
	ID       string             `json:"id"`
	Name     string             `json:"name,omitempty"`
	Color    string             `json:"color,omitempty"`
	Duration int64              `json:"duration,omitempty"`
	Genome   *Genome            `json:"genome,omitempty"`
	Tick     int64              `json:"tick"`
	Sequence int                `json:"sequence"`
	Type     string             `json:"type"`     // add_inoculum, set_flow, set_noise, impulse, perturbation, depletion, toggle_mutations
	TargetID string             `json:"targetId"` // world / colonyId / individualId
	Value    float64            `json:"value"`
	Params   map[string]float64 `json:"params,omitempty"`
}

// EnergyBalance — баланс энергии системы (раздел 11 ТЗ v2)
type EnergyBalance struct {
	InitialStored    float64 `json:"initialStored"`    // Начальный запас зародышей (E + kB*B)
	Inoculated       float64 `json:"inoculated"`       // Внесено внешними воздействиями исследователя
	ExternalInput    float64 `json:"externalInput"`    // Принято от среды
	CurrentStored    float64 `json:"currentStored"`    // Запас живых особей
	InTransit        float64 `json:"inTransit"`        // Энергия полезных пакетов в пути
	Maintenance      float64 `json:"maintenance"`      // Израсходовано на поддержание жизни
	Signaling        float64 `json:"signaling"`        // Израсходовано на отправку сигналов
	ChannelLoss      float64 `json:"channelLoss"`      // Потери в каналах
	ConversionLoss   float64 `json:"conversionLoss"`   // Потери при конверсии в структуру
	DivisionCost     float64 `json:"divisionCost"`     // Расходы на деление
	Overflow         float64 `json:"overflow"`         // Рассеяно при переполнении Emax
	DeathDissipation float64 `json:"deathDissipation"` // Рассеяно при гибели особей
}

// MetricsSnapshot — метрики состояния системы на такте (раздел 10 ТЗ v2)
type MetricsSnapshot struct {
	Tick              int64   `json:"tick"`
	TimeTU            float64 `json:"timeTU"`
	Population        int     `json:"population"`      // Число живых особей
	ActiveColonies    int     `json:"activeColonies"`  // Число активных колоний
	SurvivalRate      float64 `json:"survivalRate"`    // Доля выживших исходных особей (%)
	InputPower        float64 `json:"inputPower"`      // Принятая внешняя мощность / dt
	UsefulPower       float64 `json:"usefulPower"`     // Полезная мощность (поддержание + рост) / dt
	Efficiency        float64 `json:"efficiency"`      // Энергоэффективность (%)
	DecisionEntropy   float64 `json:"decisionEntropy"` // Информационная энтропия решений (биты 0..2)
	DeliveryLatency   float64 `json:"deliveryLatency"` // Средняя задержка доставки пакетов
	ResponseLatency   float64 `json:"responseLatency"` // Задержка ответа на вмешательство
	DeliveryMeasured  bool    `json:"deliveryMeasured"`
	ResponseMeasured  bool    `json:"responseMeasured"`
	BirthsTotal       int     `json:"birthsTotal"`       // Всего рождений особей
	ColonySplitsTotal int     `json:"colonySplitsTotal"` // Всего отделений колоний
	DeathsTotal       int     `json:"deathsTotal"`       // Всего смертей особей
	MeanWelfare       float64 `json:"meanWelfare"`       // Средний уровень запаса относительно целевого (%)
	BalanceResidual   float64 `json:"balanceResidual"`   // Невязка уравнения баланса энергии
}

// StateSnapshot — полный авторитетный снимок мира и жизни (раздел 13 ТЗ v2)
type StateSnapshot struct {
	Mode          Mode             `json:"mode"`
	Flow          float64          `json:"flow"`
	Noise         float64          `json:"noise"`
	Interventions []*Intervention  `json:"interventions"`
	Tick          int64            `json:"tick"`
	Revision      int64            `json:"revision"`
	Checksum      string           `json:"checksum"` // SHA-256 состояния
	Status        ExperimentStatus `json:"status"`
	World         World            `json:"world"`
	Individuals   []Individual     `json:"individuals"`
	Colonies      []Colony         `json:"colonies"`
	Channels      []Channel        `json:"channels"`
	InTransit     []ResourcePacket `json:"inTransit"`
	Signals       []SignalMessage  `json:"signals"`
	Metrics       MetricsSnapshot  `json:"metrics"`
	Balance       EnergyBalance    `json:"balance"`
}

package model

// Parameters — численные параметры симуляции согласно разделу 7.1 и приложению A.1 ТЗ
type Parameters struct {
	// Время и дискретизация
	Dt float64 `json:"dt"` // Шаг времени TU (по умолчанию 0.1)

	// Ограничения популяции
	MaxPopulation int `json:"maxPopulation"` // Предел популяции (по умолчанию 100)

	// Параметры колонии
	EMax               float64 `json:"eMax"`               // Максимальная ёмкость энергии (EU, 100)
	InitialEnergy      float64 `json:"initialEnergy"`      // Начальный запас энергии (EU, 35)
	ComplianceC        float64 `json:"complianceC"`        // Податливость C (VU/PU, 2.0)
	MaintenancePowerPm float64 `json:"maintenancePowerPm"` // Поддержание Pm (EU/TU, 0.5)
	StarvationLimit    int     `json:"starvationLimit"`    // Тактов дефицита до гибели (20)

	// Каналы связи и передача
	MaxChannelPower float64 `json:"maxChannelPower"` // Предельная мощность канала (EU/TU, 2.0)
	ConductanceG    float64 `json:"conductanceG"`    // Проводимость G (0.05)
	ChannelLoss     float64 `json:"channelLoss"`     // Потери канала (0.10, т.е. 10%)
	ChannelDelay    int     `json:"channelDelay"`    // Задержка доставки (2 такта)

	// Рост и воспроизводство
	BSplit             float64 `json:"bSplit"`             // Порог структуры для деления (BU, 20)
	KB                 float64 `json:"kB"`                 // Конверсия энергии в биомассу (1.0 EU/BU)
	EtaGrowth          float64 `json:"etaGrowth"`          // Эффективность конверсии (0.80)
	PGrowthMax         float64 `json:"pGrowthMax"`         // Максимальная скорость роста (EU/TU, 2.0)
	ChildInitialEnergy float64 `json:"childInitialEnergy"` // Стартовая энергия потомка (EU, 10)
	DivisionCost       float64 `json:"divisionCost"`       // Безвозвратная стоимость деления (EU, 2)
	DivisionCooldown   int64   `json:"divisionCooldown"`   // Ожидание между делениями (тактов, 100)

	// Сигналы и наблюдение
	SignalCostPerChannel float64 `json:"signalCostPerChannel"` // Стоимость отправки одного сигнала (EU, 0.001)
	SignalTTL            int64   `json:"signalTtl"`            // Время жизни сигнала (тактов, 20)
	SignalPeriod         int64   `json:"signalPeriod"`         // Период рассылки сигналов (тактов, 5)
	ReserveTarget        float64 `json:"reserveTarget"`        // Целевой резерв безопасности (EU, 10)

	// Веса функции полезности по умолчанию
	H                 float64 `json:"h"`                 // Порог переключения с действия STORE (0.005)
	BaseEnergyWeight  float64 `json:"baseEnergyWeight"`  // Базовый вес накопления энергии (0.20)
	BaseSafety        float64 `json:"baseSafety"`        // Базовый вес безопасности (0.35)
	BaseCostWeight    float64 `json:"baseCostWeight"`    // Базовый вес стоимости (0.05)
	ReferenceInflow   float64 `json:"referenceInflow"`   // Опорный приток для нормирования памяти (1.0 EU/такт)
	NoiseAmplitude    float64 `json:"noiseAmplitude"`    // Амплитуда внешнего шума источников (0.05)

	// Мутации (эволюционный режим)
	MutationProbability float64 `json:"mutationProbability"` // Вероятность мутации гена (0.10)
	MutationAmplitude   float64 `json:"mutationAmplitude"`   // Макс. изменение гена (0.05 от ширины)
}

// DefaultParameters возвращает канонические параметры MVP согласно разделу 7.1
func DefaultParameters() Parameters {
	return Parameters{
		Dt:                   0.1,
		MaxPopulation:        100,
		EMax:                 100.0,
		InitialEnergy:        35.0,
		ComplianceC:          2.0,
		MaintenancePowerPm:   0.5,
		StarvationLimit:      20,
		MaxChannelPower:      2.0,
		ConductanceG:         0.05,
		ChannelLoss:          0.10,
		ChannelDelay:         2,
		BSplit:               20.0,
		KB:                   1.0,
		EtaGrowth:            0.80,
		PGrowthMax:           2.0,
		ChildInitialEnergy:   10.0,
		DivisionCost:         2.0,
		DivisionCooldown:     100,
		SignalCostPerChannel: 0.001,
		SignalTTL:            20,
		SignalPeriod:         5,
		ReserveTarget:        10.0,
		H:                    0.005,
		BaseEnergyWeight:     0.20,
		BaseSafety:           0.35,
		BaseCostWeight:       0.05,
		ReferenceInflow:      1.0,
		NoiseAmplitude:       0.05,
		MutationProbability:  0.10,
		MutationAmplitude:    0.05,
	}
}

// DefaultGenome возвращает канонический геном особи согласно ТЗ v2 (раздел 7.3)
func DefaultGenome() Genome {
	return Genome{
		WeightEnergy:       0.30,
		WeightDeficit:      0.25,
		WeightRelief:       0.20,
		WeightReproduction: 0.15,
		WeightCost:         0.10,
		Lambda:             0.80,
		HThreshold:         0.05,
	}
}

package environment

import (
	"math"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// MarsEnvironment — модуль среды Марса: пылевые резонаторы (раздел 6 ТЗ v2)
// Модель: движение частиц порождает трибоэлектрические импульсы в разреженной атмосфере.
// Буря усиливает шум, затухание в каналах и затраты на поддержание структур от абразивного износа.
type MarsEnvironment struct {
	world           model.World
	flowMultiplier  float64
	noiseMultiplier float64
	dustIntensity   float64
	stormFactor     float64
}

// NewMarsEnvironment создает экземпляр среды Марса
func NewMarsEnvironment(world model.World) *MarsEnvironment {
	dust := world.Scenario.SpecificParams["dustIntensity"]
	if dust <= 0 {
		dust = 1.5
	}
	storm := world.Scenario.SpecificParams["stormFactor"]
	if storm <= 0 {
		storm = 1.0
	}

	return &MarsEnvironment{
		world:           world,
		flowMultiplier:  1.0,
		noiseMultiplier: 1.0,
		dustIntensity:   dust,
		stormFactor:     storm,
	}
}

func (m *MarsEnvironment) WorldID() string {
	return "mars"
}

func (m *MarsEnvironment) SampleField(lat, lng float64, tick int64) float64 {
	// Интенсивность ветра и трибоэлектрических микрозарядов
	latRad := lat * math.Pi / 180.0
	lngRad := lng * math.Pi / 180.0
	phase := float64(tick%200) * math.Pi / 100.0

	// Сезонная циркуляция в бассейне Ацидалии
	circulation := math.Cos(latRad) * (1.0 + 0.5*math.Sin(lngRad+phase))
	return math.Max(0.0, m.dustIntensity*m.stormFactor*circulation)
}

func (m *MarsEnvironment) ResourceInput(ind *model.Individual, dt float64, noise float64) float64 {
	flux := m.SampleField(ind.Lat, ind.Lng, ind.Age)
	// Резонансный захват трибоэлектрических импульсов
	noiseFactor := 1.0 + m.world.Scenario.NoiseAmplitude*m.noiseMultiplier*(2.0*noise-1.0)
	noiseFactor = math.Max(0.0, noiseFactor)

	actualPower := flux * m.world.Scenario.BaseFlow * m.flowMultiplier * noiseFactor
	return math.Max(0.0, actualPower*dt)
}

func (m *MarsEnvironment) MaintenanceCost(ind *model.Individual, dt float64) float64 {
	// Базовое поддержание + абразивный износ от пылевых частиц во время бури
	abrasiveWear := 0.3 * (m.stormFactor - 1.0)
	abrasiveWear = math.Max(0.0, abrasiveWear)
	return (m.world.Model.MaintenanceRate + abrasiveWear) * dt
}

func (m *MarsEnvironment) ChannelProperties(a, b *model.Individual) (distance float64, loss float64, delay int) {
	distRad := GreatCircleDistance(a.Lat, a.Lng, b.Lat, b.Lng)
	// В запыленной атмосфере затухание сильнее зависит от расстояния и фактора бури
	loss = math.Min(0.9, 0.08*m.stormFactor+0.7*distRad)
	delay = 1
	if distRad > 0.25 {
		delay = 2
	}
	return distRad, loss, delay
}

func (m *MarsEnvironment) ApplyIntervention(it *model.Intervention) {
	switch it.Type {
	case "set_flow":
		m.flowMultiplier = math.Max(0.0, it.Value)
	case "set_noise":
		m.noiseMultiplier = math.Max(0.0, it.Value)
	case "impulse":
		m.flowMultiplier = 2.0
	case "perturbation":
		// Пылевая буря: резкий рост шума и абразивного износа
		m.noiseMultiplier = 3.5
		m.stormFactor = 2.5
	case "depletion":
		m.flowMultiplier = 0.0
	}
}

func (m *MarsEnvironment) GetFlowMultiplier() float64 {
	return m.flowMultiplier
}

func (m *MarsEnvironment) GetNoiseMultiplier() float64 {
	return m.noiseMultiplier
}

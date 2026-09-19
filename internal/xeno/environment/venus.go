package environment

import (
	"math"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// VenusEnvironment — модуль среды Венеры: тепловые структуры (раздел 6 ТЗ v2)
// Модель: Qdot = k A ΔT / L, оба резервуара имеют температуру в Кельвинах.
// Верхняя граница полезной работы не превышает предел Карно: η ≤ 1 - Tc/Th.
// В сверхплотной атмосфере (92 бар) высоки затраты на отвод тепла и сохранение структуры.
type VenusEnvironment struct {
	world           model.World
	flowMultiplier  float64
	noiseMultiplier float64
	hotReservoirK   float64
	coldReservoirK  float64
	carnotLimit     float64
}

// NewVenusEnvironment создает экземпляр среды Венеры
func NewVenusEnvironment(world model.World) *VenusEnvironment {
	th := world.Scenario.SpecificParams["hotReservoirK"]
	if th <= 0 {
		th = 750.0
	}
	tc := world.Scenario.SpecificParams["coldReservoirK"]
	if tc <= 0 {
		tc = 710.0
	}
	carnot := 1.0 - (tc / th)
	if carnot <= 0 {
		carnot = 0.053
	}

	return &VenusEnvironment{
		world:           world,
		flowMultiplier:  1.0,
		noiseMultiplier: 1.0,
		hotReservoirK:   th,
		coldReservoirK:  tc,
		carnotLimit:     carnot,
	}
}

func (v *VenusEnvironment) WorldID() string {
	return "venus"
}

func (v *VenusEnvironment) SampleField(lat, lng float64, tick int64) float64 {
	// Локальный тепловой поток через вертикальный градиент
	latRad := lat * math.Pi / 180.0
	lngRad := lng * math.Pi / 180.0
	phase := float64(tick%500) * math.Pi / 250.0

	// Медленная конвекция в сверхплотной атмосфере
	convection := 1.0 + 0.2*math.Cos(latRad)*math.Sin(lngRad+phase)
	deltaT := (v.hotReservoirK - v.coldReservoirK) * convection
	return math.Max(0.0, deltaT)
}

func (v *VenusEnvironment) ResourceInput(ind *model.Individual, dt float64, noise float64) float64 {
	deltaT := v.SampleField(ind.Lat, ind.Lng, ind.Age)
	// Тепловой поток Qdot = k * A * deltaT / L
	// Полезная работа ограничена термодинамическим пределом Карно: W = carnot * Qdot
	qDot := deltaT * 0.25
	usefulPower := qDot * v.carnotLimit * v.world.Scenario.BaseFlow

	noiseFactor := 1.0 + v.world.Scenario.NoiseAmplitude*v.noiseMultiplier*(2.0*noise-1.0)
	noiseFactor = math.Max(0.0, noiseFactor)

	actualPower := usefulPower * v.flowMultiplier * noiseFactor
	return math.Max(0.0, actualPower*dt)
}

func (v *VenusEnvironment) MaintenanceCost(ind *model.Individual, dt float64) float64 {
	// В условиях 464°C и 92 бар требуются высокие затраты на охлаждение и сброс энтропии
	coolingOverhead := 0.25 * (ind.Energy / v.world.Model.EMax)
	return (v.world.Model.MaintenanceRate + coolingOverhead) * dt
}

func (v *VenusEnvironment) ChannelProperties(a, b *model.Individual) (distance float64, loss float64, delay int) {
	distRad := GreatCircleDistance(a.Lat, a.Lng, b.Lat, b.Lng)
	// В плотной среде задержка сигнала выше из-за рассеяния
	loss = math.Min(0.9, 0.06+0.6*distRad)
	delay = 2
	if distRad > 0.4 {
		delay = 3
	}
	return distRad, loss, delay
}

func (v *VenusEnvironment) ApplyIntervention(it *model.Intervention) {
	switch it.Type {
	case "set_flow":
		v.flowMultiplier = math.Max(0.0, it.Value)
	case "set_noise":
		v.noiseMultiplier = math.Max(0.0, it.Value)
	case "impulse":
		// Локальный термальный выброс
		v.flowMultiplier = 2.2
	case "perturbation":
		// Атмосферная волна давления: усиление тепловых флуктуаций
		v.noiseMultiplier = 2.8
	case "depletion":
		v.flowMultiplier = 0.0
	}
}

func (v *VenusEnvironment) GetFlowMultiplier() float64 {
	return v.flowMultiplier
}

func (v *VenusEnvironment) GetNoiseMultiplier() float64 {
	return v.noiseMultiplier
}

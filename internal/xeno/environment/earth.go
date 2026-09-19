package environment

import (
	"math"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// EarthEnvironment — модуль среды Земли: минеральные проводящие структуры (раздел 6 ТЗ v2)
// Модель: внешний электрический градиент питает сеть накопителей заряда.
// E = C V² / 2, I = G(V_ext - V_i), омические потери в проводящей минеральной сети.
type EarthEnvironment struct {
	world           model.World
	flowMultiplier  float64
	noiseMultiplier float64
	capacitanceC    float64
	conductanceG    float64
	baseGradient    float64
}

// NewEarthEnvironment создает экземпляр среды Земли
func NewEarthEnvironment(world model.World) *EarthEnvironment {
	capC := world.Scenario.SpecificParams["capacitance"]
	if capC <= 0 {
		capC = 2.0
	}
	grad := world.Scenario.SpecificParams["electricGradient"]
	if grad <= 0 {
		grad = 5.0
	}

	return &EarthEnvironment{
		world:           world,
		flowMultiplier:  1.0,
		noiseMultiplier: 1.0,
		capacitanceC:    capC,
		conductanceG:    1.5,
		baseGradient:    grad,
	}
}

func (e *EarthEnvironment) WorldID() string {
	return "earth"
}

func (e *EarthEnvironment) SampleField(lat, lng float64, tick int64) float64 {
	// Потенциал внешнего электрического градиента с гармонической вариацией
	latRad := lat * math.Pi / 180.0
	lngRad := lng * math.Pi / 180.0
	phase := float64(tick%360) * math.Pi / 180.0

	// Градиент между полярными и экваториальными минеральными пластами
	spatialVar := math.Sin(2.0*latRad) * math.Cos(lngRad+phase)
	return math.Max(0.0, e.baseGradient*(1.0+0.3*spatialVar))
}

func (e *EarthEnvironment) ResourceInput(ind *model.Individual, dt float64, noise float64) float64 {
	vExt := e.SampleField(ind.Lat, ind.Lng, ind.Age)
	// Внутренний потенциал особи: V = sqrt(2E/C)
	vInd := math.Sqrt(math.Max(0.0, 2.0*ind.Energy/e.capacitanceC))

	// Разность потенциалов обеспечивает переток тока: I = G * (V_ext - V_i)
	deltaV := math.Max(0.0, vExt-vInd)
	current := e.conductanceG * deltaV

	// Энергия заряда за интервал времени dt
	power := current * vExt
	noiseFactor := 1.0 + e.world.Scenario.NoiseAmplitude*e.noiseMultiplier*(2.0*noise-1.0)
	noiseFactor = math.Max(0.0, noiseFactor)

	actualPower := power * e.flowMultiplier * noiseFactor
	return math.Max(0.0, actualPower*dt)
}

func (e *EarthEnvironment) MaintenanceCost(ind *model.Individual, dt float64) float64 {
	// Базовое рассеяние заряда + поддержание структуры (утечки диэлектрика)
	leakageRate := 0.2 * (ind.Energy / e.world.Model.EMax)
	return (e.world.Model.MaintenanceRate + leakageRate) * dt
}

func (e *EarthEnvironment) ChannelProperties(a, b *model.Individual) (distance float64, loss float64, delay int) {
	distRad := GreatCircleDistance(a.Lat, a.Lng, b.Lat, b.Lng)
	// Омические потери растут с расстоянием: loss = min(0.9, 0.05 + 0.5 * distRad)
	loss = math.Min(0.9, 0.05+0.5*distRad)
	delay = 1
	if distRad > 0.3 {
		delay = 2
	}
	return distRad, loss, delay
}

func (e *EarthEnvironment) ApplyIntervention(it *model.Intervention) {
	switch it.Type {
	case "set_flow":
		e.flowMultiplier = math.Max(0.0, it.Value)
	case "set_noise":
		e.noiseMultiplier = math.Max(0.0, it.Value)
	case "impulse":
		e.flowMultiplier = 2.5
	case "perturbation":
		e.noiseMultiplier = 3.0
	case "depletion":
		e.flowMultiplier = 0.0
	}
}

func (e *EarthEnvironment) GetFlowMultiplier() float64 {
	return e.flowMultiplier
}

func (e *EarthEnvironment) GetNoiseMultiplier() float64 {
	return e.noiseMultiplier
}

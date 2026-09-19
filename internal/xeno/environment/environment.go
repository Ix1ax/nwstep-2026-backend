package environment

import (
	"math"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// EnvironmentModule — контракт физического модуля среды планеты (раздел 6 ТЗ v2)
type EnvironmentModule interface {
	// WorldID возвращает идентификатор мира (earth, mars, venus)
	WorldID() string

	// SampleField вычисляет значение локального физического поля в точке (lat, lng) на сфере
	SampleField(lat, lng float64, tick int64) float64

	// ResourceInput рассчитывает приток внешней энергии для особи за интервал dt с учетом шума среды
	ResourceInput(ind *model.Individual, dt float64, noise float64) float64

	// MaintenanceCost рассчитывает энергозатраты особи на сохранение структуры за dt
	MaintenanceCost(ind *model.Individual, dt float64) float64

	// ChannelProperties вычисляет свойства физической связи между двумя особями (расстояние, потери, задержка)
	ChannelProperties(a, b *model.Individual) (distance float64, loss float64, delay int)

	// ApplyIntervention применяет внешнее воздействие исследователя к параметрам среды
	ApplyIntervention(it *model.Intervention)

	// GetFlowMultiplier возвращает текущий множитель притока (с учетом импульсов/истощений)
	GetFlowMultiplier() float64

	// GetNoiseMultiplier возвращает текущий множитель шума (с учетом возмущений)
	GetNoiseMultiplier() float64
}

// GreatCircleDistance вычисляет угловое ортодромное расстояние на сфере в радианах
func GreatCircleDistance(lat1Deg, lng1Deg, lat2Deg, lng2Deg float64) float64 {
	rad := math.Pi / 180.0
	phi1 := lat1Deg * rad
	phi2 := lat2Deg * rad
	dPhi := (lat2Deg - lat1Deg) * rad
	dLambda := (lng2Deg - lng1Deg) * rad

	a := math.Sin(dPhi/2.0)*math.Sin(dPhi/2.0) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2.0)*math.Sin(dLambda/2.0)
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return c // расстояние в радианах по сфере
}

// NewEnvironmentModule создает модуль среды для указанного мира
func NewEnvironmentModule(world model.World) EnvironmentModule {
	switch world.ID {
	case "earth":
		return NewEarthEnvironment(world)
	case "mars":
		return NewMarsEnvironment(world)
	case "venus":
		return NewVenusEnvironment(world)
	default:
		return NewEarthEnvironment(world)
	}
}

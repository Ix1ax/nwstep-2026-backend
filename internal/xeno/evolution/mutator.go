package evolution

import (
	"fmt"
	"math"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/prng"
)

// Mutator реализует наследование генома, мутации и размещение особей на сфере (разделы 7.5 и 7.6 ТЗ v2)
type Mutator struct {
	params model.Parameters
}

// NewMutator создает экземпляр эволюционного модуля
func NewMutator(params model.Parameters) *Mutator {
	return &Mutator{params: params}
}

// InheritGenome наследует геном родителя с возможными мутациями (раздел 7.5 ТЗ v2)
// Два вызова RNG мутаций на каждый ген выполняются всегда для сохранения детерминизма потока.
func (m *Mutator) InheritGenome(parentGenome model.Genome, mode model.Mode, streams *prng.Streams) model.Genome {
	child := parentGenome

	mutateGene := func(val, min, max float64) float64 {
		uFact := streams.Mutations.NextFloat64()
		uDelta := streams.Mutations.NextFloat64()

		if mode != model.ModeEvolutionary {
			return val // Мутации отключены в реактивном и адаптивном режимах
		}

		if uFact < m.params.MutationProbability {
			width := max - min
			delta := (2.0*uDelta - 1.0) * m.params.MutationAmplitude * width
			newVal := val + delta
			if newVal < min {
				newVal = min
			}
			if newVal > max {
				newVal = max
			}
			return newVal
		}
		return val
	}

	child.WeightEnergy = mutateGene(child.WeightEnergy, 0.05, 0.90)
	child.WeightDeficit = mutateGene(child.WeightDeficit, 0.05, 0.90)
	child.WeightRelief = mutateGene(child.WeightRelief, 0.00, 0.90)
	child.WeightReproduction = mutateGene(child.WeightReproduction, 0.00, 0.90)
	child.WeightCost = mutateGene(child.WeightCost, 0.01, 0.50)
	child.Lambda = mutateGene(child.Lambda, 0.10, 0.99)
	child.HThreshold = mutateGene(child.HThreshold, 0.01, 0.20)

	return child
}

// FindChildPosition находит свободное место на сфере (lat, lng) вокруг родителя
// Использует 8 направлений на угловом расстоянии deltaAngleRad (~0.05 радиан ≈ 2.8 градуса)
func (m *Mutator) FindChildPosition(
	parentLat, parentLng float64,
	existingIndividuals []*model.Individual,
	streams *prng.Streams,
) (float64, float64, bool) {
	deltaAngle := 3.0   // 3 градуса на сфере
	minDistRad := 0.025 // минимальное угловое расстояние (~1.4 градуса)

	// 8 азимутов (N, NE, E, SE, S, SW, W, NW)
	angles := []float64{0, 45, 90, 135, 180, 225, 270, 315}

	// Детерминированный сдвиг начального угла
	offset := int(streams.Placement.NextFloat64() * 8.0)
	if offset < 0 {
		offset = 0
	}
	if offset >= 8 {
		offset = 7
	}

	for i := 0; i < 8; i++ {
		idx := (offset + i) % 8
		azimuthRad := angles[idx] * math.Pi / 180.0

		dLat := deltaAngle * math.Cos(azimuthRad)
		cosLat := math.Cos(parentLat * math.Pi / 180.0)
		if math.Abs(cosLat) < 0.01 {
			cosLat = 0.01
		}
		dLng := (deltaAngle * math.Sin(azimuthRad)) / cosLat

		candLat := parentLat + dLat
		candLng := parentLng + dLng

		// Нормализация координат на сфере
		if candLat > 85.0 {
			candLat = 85.0
		}
		if candLat < -85.0 {
			candLat = -85.0
		}
		for candLng > 180.0 {
			candLng -= 360.0
		}
		for candLng < -180.0 {
			candLng += 360.0
		}

		// Проверка коллизий со всеми живыми особями
		collides := false
		for _, ind := range existingIndividuals {
			if !ind.Alive {
				continue
			}
			dist := environment.GreatCircleDistance(candLat, candLng, ind.Lat, ind.Lng)
			if dist < minDistRad {
				collides = true
				break
			}
		}

		if !collides {
			return candLat, candLng, true
		}
	}

	return 0, 0, false
}

// CheckColonyBudding проверяет условия образования дочерней колонии (раздел 7.6 ТЗ v2)
// При достижении 18 особей и устойчивости ≥25 тактов половина особей отделяется в новую колонию.
func (m *Mutator) CheckColonyBudding(
	colony *model.Colony,
	individuals map[string]*model.Individual,
	tick int64,
	nextColonyNum int,
) (*model.Colony, []string, bool) {
	liveCount := 0
	liveIDs := make([]string, 0)
	for _, id := range colony.IndividualIDs {
		ind, ok := individuals[id]
		if ok && ind.Alive {
			liveCount++
			liveIDs = append(liveIDs, id)
		}
	}

	// Условие почкования: не менее 18 живых особей и не менее 25 тактов с момента образования
	if liveCount < 18 || (tick-colony.FormedAtTick) < 25 {
		return nil, nil, false
	}

	// Отделяем ровно половину в дочернюю колонию
	splitSize := liveCount / 2
	daughterIDs := liveIDs[splitSize:]
	remainingIDs := liveIDs[:splitSize]

	daughterColonyID := fmt.Sprintf("colony-%02d", nextColonyNum)
	colors := []string{"#3b82f6", "#10b981", "#f59e0b", "#ec4899", "#8b5cf6", "#06b6d4", "#f97316"}
	colorIdx := nextColonyNum % len(colors)

	daughterColony := &model.Colony{
		ID:             daughterColonyID,
		WorldID:        colony.WorldID,
		ParentColonyID: &colony.ID,
		Name:           fmt.Sprintf("%s-Дочерняя", colony.Name),
		Color:          colors[colorIdx],
		FormedAtTick:   tick,
		IndividualIDs:  daughterIDs,
		Metrics: model.ColonyMetrics{
			Population: len(daughterIDs),
		},
	}

	return daughterColony, remainingIDs, true
}

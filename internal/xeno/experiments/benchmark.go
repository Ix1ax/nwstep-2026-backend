package experiments

import (
	"fmt"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// BuildWorldScenario создает канонический сценарий для выбранной планеты согласно ТЗ v2
// Формирует 2 начальные колонии по 6 особей со сферическими координатами и связями.
func BuildWorldScenario(
	worldID string,
	mode model.Mode,
	seed uint64,
	params model.Parameters,
) (*model.StateSnapshot, []*model.Intervention) {
	world, ok := model.FindWorldByID(worldID)
	if !ok {
		wList := model.GetPresetWorlds()
		world = &wList[0] // fallback to Earth
	}

	individuals := make([]model.Individual, 0, 12)
	colonies := make([]model.Colony, 0, 2)
	channels := make([]model.Channel, 0, 24)

	genome := model.DefaultGenome()

	// 1. Колония 1: «Северное сообщество» (широта ~20°, долгота ~10°)
	col1IndIDs := make([]string, 0, 6)
	col1Coords := [][2]float64{
		{20.0, 10.0}, {22.0, 10.5}, {24.0, 11.0},
		{20.5, 13.0}, {22.5, 13.5}, {24.5, 14.0},
	}
	for i, coord := range col1Coords {
		id := fmt.Sprintf("ind-%02d", i+1)
		col1IndIDs = append(col1IndIDs, id)
		ind := model.Individual{
			ID:               id,
			WorldID:          world.ID,
			ColonyID:         "colony-01",
			Lat:              coord[0],
			Lng:              coord[1],
			Energy:           params.InitialEnergy,
			Biomass:          5.0,
			Memory:           0.0,
			Genome:           genome,
			Alive:            true,
			Age:              0,
			Generation:       1,
			BirthTick:        0,
			LastDivisionTick: 0,
		}
		individuals = append(individuals, ind)
	}

	col1 := model.Colony{
		ID:            "colony-01",
		WorldID:       world.ID,
		Name:          "Северное Сообщество",
		Color:         "#3b82f6", // Синий
		FormedAtTick:  0,
		IndividualIDs: col1IndIDs,
		Metrics: model.ColonyMetrics{
			Population:   len(col1IndIDs),
			TotalEnergy:  float64(len(col1IndIDs)) * params.InitialEnergy,
			TotalBiomass: float64(len(col1IndIDs)) * 5.0,
		},
	}
	colonies = append(colonies, col1)

	// 2. Колония 2: «Экваториальное сообщество» (широта ~5°, долгота ~25°)
	col2IndIDs := make([]string, 0, 6)
	col2Coords := [][2]float64{
		{5.0, 25.0}, {7.0, 25.5}, {9.0, 26.0},
		{5.5, 28.0}, {7.5, 28.5}, {9.5, 29.0},
	}
	for i, coord := range col2Coords {
		id := fmt.Sprintf("ind-%02d", i+7)
		col2IndIDs = append(col2IndIDs, id)
		ind := model.Individual{
			ID:               id,
			WorldID:          world.ID,
			ColonyID:         "colony-02",
			Lat:              coord[0],
			Lng:              coord[1],
			Energy:           params.InitialEnergy,
			Biomass:          5.0,
			Memory:           0.0,
			Genome:           genome,
			Alive:            true,
			Age:              0,
			Generation:       1,
			BirthTick:        0,
			LastDivisionTick: 0,
		}
		individuals = append(individuals, ind)
	}

	col2 := model.Colony{
		ID:            "colony-02",
		WorldID:       world.ID,
		Name:          "Экваториальное Сообщество",
		Color:         "#10b981", // Зеленый
		FormedAtTick:  0,
		IndividualIDs: col2IndIDs,
		Metrics: model.ColonyMetrics{
			Population:   len(col2IndIDs),
			TotalEnergy:  float64(len(col2IndIDs)) * params.InitialEnergy,
			TotalBiomass: float64(len(col2IndIDs)) * 5.0,
		},
	}
	colonies = append(colonies, col2)

	// 3. Создаем физические каналы связи
	addChannelPair := func(from, to model.Individual) {
		dist, loss, delay := environment.NewEnvironmentModule(*world).ChannelProperties(&from, &to)

		channels = append(channels, model.Channel{
			ID:          fmt.Sprintf("ch-%s-%s", from.ID, to.ID),
			FromID:      from.ID,
			ToID:        to.ID,
			Distance:    dist,
			Conductance: params.ConductanceG,
			MaxPower:    params.MaxChannelPower,
			Loss:        loss,
			DelayTicks:  delay,
			Enabled:     true,
		})
		channels = append(channels, model.Channel{
			ID:          fmt.Sprintf("ch-%s-%s", to.ID, from.ID),
			FromID:      to.ID,
			ToID:        from.ID,
			Distance:    dist,
			Conductance: params.ConductanceG,
			MaxPower:    params.MaxChannelPower,
			Loss:        loss,
			DelayTicks:  delay,
			Enabled:     true,
		})
	}

	// Связи внутри колонии 1
	addChannelPair(individuals[0], individuals[1])
	addChannelPair(individuals[1], individuals[2])
	addChannelPair(individuals[3], individuals[4])
	addChannelPair(individuals[4], individuals[5])
	addChannelPair(individuals[0], individuals[3])
	addChannelPair(individuals[1], individuals[4])
	addChannelPair(individuals[2], individuals[5])

	// Связи внутри колонии 2
	addChannelPair(individuals[6], individuals[7])
	addChannelPair(individuals[7], individuals[8])
	addChannelPair(individuals[9], individuals[10])
	addChannelPair(individuals[10], individuals[11])
	addChannelPair(individuals[6], individuals[9])
	addChannelPair(individuals[7], individuals[10])
	addChannelPair(individuals[8], individuals[11])

	// Межколониальный мост связи
	addChannelPair(individuals[2], individuals[6])
	addChannelPair(individuals[5], individuals[9])

	// 4. Расписание контролируемых воздействий (раздел 8 ТЗ v2)
	interventions := []*model.Intervention{}

	totalInitialStored := 0.0
	for _, ind := range individuals {
		totalInitialStored += (ind.Energy + params.KB*ind.Biomass)
	}

	snapshot := &model.StateSnapshot{
		Tick:        0,
		Revision:    1,
		Status:      model.StatusReady,
		World:       *world,
		Individuals: individuals,
		Colonies:    colonies,
		Channels:    channels,
		InTransit:   []model.ResourcePacket{},
		Signals:     []model.SignalMessage{},
		Balance: model.EnergyBalance{
			InitialStored: totalInitialStored,
			CurrentStored: totalInitialStored,
		},
		Metrics: model.MetricsSnapshot{
			Tick:           0,
			TimeTU:         0.0,
			Population:     len(individuals),
			ActiveColonies: len(colonies),
			SurvivalRate:   100.0,
		},
	}

	return snapshot, interventions
}

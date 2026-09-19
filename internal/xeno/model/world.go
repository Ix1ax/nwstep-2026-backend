package model

// PlanetaryReference — справочные физические величины из опубликованных данных (NASA Planetary Fact Sheet)
type PlanetaryReference struct {
	BodyName        string  `json:"bodyName"`
	MeanTemperature float64 `json:"meanTemperatureCelsius"` // °C
	TemperatureK    float64 `json:"temperatureKelvin"`      // K
	Gravity         float64 `json:"gravity"`                // м/с²
	SurfacePressure float64 `json:"surfacePressureBar"`     // бар
	Source          string  `json:"source"`
	SourceDate      string  `json:"sourceDate"`
}

// LocalScenarioConfig — локальный профиль условий моделируемой области (не меняет всю планету)
type LocalScenarioConfig struct {
	RegionName     string             `json:"regionName"`
	BaseFlow       float64            `json:"baseFlow"`       // Базовый приток энергии
	NoiseAmplitude float64            `json:"noiseAmplitude"` // Амплитуда флуктуаций шума
	SpecificParams map[string]float64 `json:"specificParams"` // Физически специфичные параметры (градиент, интенсивность бури, перепад температур)
}

// ModelConfig — коэффициенты абстрактного организма и правила связи
type ModelConfig struct {
	OrganismType    string  `json:"organismType"` // mineral_conductive, dust_resonator, thermal_structure
	Dt              float64 `json:"dt"`
	EMax            float64 `json:"eMax"`
	ReserveTarget   float64 `json:"reserveTarget"`
	MaintenanceRate float64 `json:"maintenanceRate"`
	StarvationLimit int     `json:"starvationLimit"`
	MaxPopulation   int     `json:"maxPopulation"`
	MaxColonies     int     `json:"maxColonies"`
}

// World — среда моделирования (небесное тело как физическая основа)
type World struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Reference   PlanetaryReference  `json:"reference"`
	Scenario    LocalScenarioConfig `json:"scenario"`
	Model       ModelConfig         `json:"model"`
	TextureURL  string              `json:"textureUrl,omitempty"`
}

// GetPresetWorlds возвращает список 3 эталонных миров MVP со справочными данными NASA Fact Sheet
func GetPresetWorlds() []World {
	return []World{
		{
			ID:          "earth",
			Name:        "Земля",
			Description: "Минеральные проводящие структуры в локальном электрическом градиенте",
			Reference: PlanetaryReference{
				BodyName:        "Earth",
				MeanTemperature: 15.0,
				TemperatureK:    288.15,
				Gravity:         9.8,
				SurfacePressure: 1.0,
				Source:          "NASA Planetary Fact Sheet (https://nssdc.gsfc.nasa.gov/planetary/factsheet/)",
				SourceDate:      "2025-03-18",
			},
			Scenario: LocalScenarioConfig{
				RegionName:     "Геотермально-минеральный разлом",
				BaseFlow:       10.0,
				NoiseAmplitude: 0.1,
				SpecificParams: map[string]float64{
					"electricGradient": 5.0, // В/м
					"capacitance":      2.0, // Фарад
				},
			},
			Model: ModelConfig{
				OrganismType:    "mineral_conductive",
				Dt:              0.1,
				EMax:            100.0,
				ReserveTarget:   40.0,
				MaintenanceRate: 1.0,
				StarvationLimit: 5,
				MaxPopulation:   2000,
				MaxColonies:     100,
			},
			TextureURL: "/textures/earth.jpg",
		},
		{
			ID:          "mars",
			Name:        "Марс",
			Description: "Пылевые резонаторы, улавливающие трибоэлектрические импульсы в разреженной атмосфере",
			Reference: PlanetaryReference{
				BodyName:        "Mars",
				MeanTemperature: -65.0,
				TemperatureK:    208.15,
				Gravity:         3.7,
				SurfacePressure: 0.01,
				Source:          "NASA Planetary Fact Sheet (https://nssdc.gsfc.nasa.gov/planetary/factsheet/)",
				SourceDate:      "2025-03-18",
			},
			Scenario: LocalScenarioConfig{
				RegionName:     "Равнина Ацидалия (пылевой бассейн)",
				BaseFlow:       8.0,
				NoiseAmplitude: 0.25, // Повышенный шум от пылевых вихрей
				SpecificParams: map[string]float64{
					"dustIntensity": 1.5, // Интенсивность движения частиц
					"stormFactor":   1.0, // Множитель бури
				},
			},
			Model: ModelConfig{
				OrganismType:    "dust_resonator",
				Dt:              0.1,
				EMax:            90.0,
				ReserveTarget:   35.0,
				MaintenanceRate: 0.8,
				StarvationLimit: 6,
				MaxPopulation:   2000,
				MaxColonies:     100,
			},
			TextureURL: "/textures/mars.jpg",
		},
		{
			ID:          "venus",
			Name:        "Венера",
			Description: "Тепловые структуры в сверхплотной атмосфере, использующие вертикальный температурный градиент",
			Reference: PlanetaryReference{
				BodyName:        "Venus",
				MeanTemperature: 464.0,
				TemperatureK:    737.15,
				Gravity:         8.9,
				SurfacePressure: 92.0,
				Source:          "NASA Planetary Fact Sheet (https://nssdc.gsfc.nasa.gov/planetary/factsheet/)",
				SourceDate:      "2025-03-18",
			},
			Scenario: LocalScenarioConfig{
				RegionName:     "Область каньона Дионы",
				BaseFlow:       12.0,
				NoiseAmplitude: 0.15,
				SpecificParams: map[string]float64{
					"hotReservoirK":  750.0, // Температура нижнего слоя (К)
					"coldReservoirK": 710.0, // Температура верхнего слоя (К)
					"carnotLimit":    0.053, // 1 - 710/750
				},
			},
			Model: ModelConfig{
				OrganismType:    "thermal_structure",
				Dt:              0.1,
				EMax:            120.0,
				ReserveTarget:   50.0,
				MaintenanceRate: 1.2,
				StarvationLimit: 4,
				MaxPopulation:   2000,
				MaxColonies:     100,
			},
			TextureURL: "/textures/venus.jpg",
		},
	}
}

// FindWorldByID находит мир по его идентификатору
func FindWorldByID(id string) (*World, bool) {
	worlds := GetPresetWorlds()
	for _, w := range worlds {
		if w.ID == id {
			return &w, true
		}
	}
	return nil, false
}

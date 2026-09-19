package colony

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	ErrColonyNotFound = errors.New("colony not found")
)

type Service struct {
	mu          sync.RWMutex
	colonies    map[string]*Colony
	environment *EnvironmentState
	log         zerolog.Logger
}

func NewService(log zerolog.Logger) *Service {
	s := &Service{
		colonies: make(map[string]*Colony),
		environment: &EnvironmentState{
			SolarRadiation:     45.0,
			AmbientTemperature: 150.0, // Kelvin
			SystemEntropy:      18.0,  // %
			FreeResourceMotes:  40,
			ActivePolicy:       "symbiosis",
			TickCount:          0,
			LastEvent:          "System initialized in baseline equilibrium",
			UpdatedAt:          time.Now(),
		},
		log: log,
	}

	s.seedColonies()
	return s
}

func (s *Service) seedColonies() {
	now := time.Now()

	s.colonies["jupiter"] = &Colony{
		ID:            "jupiter",
		Name:          "Плазменные Атоллы",
		CelestialBody: "Юпитер",
		LifeFormType:  "plasma",
		SignalType:    "radio_ionization",
		SignalValue:   "Ne = 1.4e18 см⁻³",
		Energy:        1450.0,
		MaxEnergy:     2500.0,
		Entropy:       32.0,
		Temperature:   420.0,
		OptimalTemp:   400.0,
		Population:    18,
		Weights: EvolutionWeights{
			Alpha:      1.2, // Strong drive for raw energy
			Beta:       0.3,
			Gamma:      0.5,
			Generation: 1,
		},
		Status:    "thriving",
		PartnerID: "enceladus",
		UpdatedAt: now,
	}

	s.colonies["enceladus"] = &Colony{
		ID:            "enceladus",
		Name:          "Ледяные Гейзеры",
		CelestialBody: "Энцелад",
		LifeFormType:  "cryo_geyser",
		SignalType:    "pressure_plume",
		SignalValue:   "P = 840 кПа",
		Energy:        780.0,
		MaxEnergy:     1600.0,
		Entropy:       19.0,
		Temperature:   75.0,
		OptimalTemp:   70.0,
		Population:    12,
		Weights: EvolutionWeights{
			Alpha:      0.8,
			Beta:       1.5, // Extreme vulnerability to temperature
			Gamma:      0.9,
			Generation: 1,
		},
		Status:    "stable",
		PartnerID: "mercury",
		UpdatedAt: now,
	}

	s.colonies["mercury"] = &Colony{
		ID:            "mercury",
		Name:          "Ферромагнитные Дюны",
		CelestialBody: "Меркурий",
		LifeFormType:  "ferromagnetic_dune",
		SignalType:    "magnetic_flux",
		SignalValue:   "B = 142 мТл",
		Energy:        920.0,
		MaxEnergy:     1800.0,
		Entropy:       24.0,
		Temperature:   350.0,
		OptimalTemp:   320.0,
		Population:    14,
		Weights: EvolutionWeights{
			Alpha:      1.0,
			Beta:       0.6,
			Gamma:      0.7,
			Generation: 1,
		},
		Status:    "stable",
		PartnerID: "triton",
		UpdatedAt: now,
	}

	s.colonies["triton"] = &Colony{
		ID:            "triton",
		Name:          "Акустический Разум",
		CelestialBody: "Тритон",
		LifeFormType:  "acoustic_network",
		SignalType:    "acoustic_resonance",
		SignalValue:   "f = 432 Гц",
		Energy:        650.0,
		MaxEnergy:     1400.0,
		Entropy:       14.0,
		Temperature:   38.0,
		OptimalTemp:   40.0,
		Population:    9,
		Weights: EvolutionWeights{
			Alpha:      0.7,
			Beta:       0.8,
			Gamma:      1.4, // Extreme sensitivity to entropy (noise)
			Generation: 1,
		},
		Status:    "stable",
		PartnerID: "jupiter",
		UpdatedAt: now,
	}
}

func (s *Service) GetColonies() []*Colony {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Colony, 0, len(s.colonies))
	for _, c := range s.colonies {
		// Return copy
		cp := *c
		result = append(result, &cp)
	}
	return result
}

func (s *Service) GetColony(id string) (*Colony, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.colonies[id]
	if !ok {
		return nil, ErrColonyNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *Service) GetEnvironment() *EnvironmentState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := *s.environment
	return &cp
}

func (s *Service) SetPolicy(policy string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.environment.ActivePolicy = policy
	s.environment.UpdatedAt = time.Now()
}

func (s *Service) TriggerEvent(req TriggerEventRequest) (*EnvironmentState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intensity := req.Intensity
	if intensity <= 0 {
		intensity = 1.0
	}

	now := time.Now()

	switch req.EventType {
	case "solar_flare":
		s.environment.SolarRadiation = math.Min(100.0, s.environment.SolarRadiation+30.0*intensity)
		s.environment.AmbientTemperature += 45.0 * intensity
		s.environment.SystemEntropy = math.Min(100.0, s.environment.SystemEntropy+15.0*intensity)
		s.environment.LastEvent = "Солнечная вспышка: тепловой удар по системе"

		// Jupiter & Mercury absorb energy; Enceladus overheats
		if j, ok := s.colonies["jupiter"]; ok {
			j.Energy = math.Min(j.MaxEnergy, j.Energy+400*intensity)
			j.Temperature += 20 * intensity
		}
		if m, ok := s.colonies["mercury"]; ok {
			m.Energy = math.Min(m.MaxEnergy, m.Energy+250*intensity)
			m.Temperature += 30 * intensity
		}
		if e, ok := s.colonies["enceladus"]; ok {
			e.Temperature += 40 * intensity
			e.Entropy = math.Min(100.0, e.Entropy+25.0*intensity)
			e.Status = "stressed"
		}

	case "cryo_wave":
		s.environment.AmbientTemperature = math.Max(20.0, s.environment.AmbientTemperature-50.0*intensity)
		s.environment.SystemEntropy = math.Max(5.0, s.environment.SystemEntropy-10.0*intensity)
		s.environment.LastEvent = "Крио-волна: глубокое охлаждение пространства"

		if e, ok := s.colonies["enceladus"]; ok {
			e.Energy = math.Min(e.MaxEnergy, e.Energy+200*intensity)
			e.Temperature = math.Max(40.0, e.Temperature-30*intensity)
			e.Status = "thriving"
		}
		if j, ok := s.colonies["jupiter"]; ok {
			j.Energy = math.Max(100.0, j.Energy-150*intensity)
		}

	case "em_pulse":
		s.environment.SystemEntropy = math.Max(5.0, s.environment.SystemEntropy-12.0*intensity)
		s.environment.LastEvent = "ЭМИ-импульс: резонансная гармонизация полей"

		if m, ok := s.colonies["mercury"]; ok {
			m.Energy = math.Min(m.MaxEnergy, m.Energy+180*intensity)
		}
		if t, ok := s.colonies["triton"]; ok {
			t.Energy = math.Min(t.MaxEnergy, t.Energy+150*intensity)
			t.Entropy = math.Max(5.0, t.Entropy-10.0*intensity)
		}

	case "resource_cluster":
		s.environment.FreeResourceMotes += int(25 * intensity)
		s.environment.LastEvent = "Рой ресурсов: выброс ионно-солевых градиентов"

	default:
		s.environment.LastEvent = "Исследовательское сканирование среды"
	}

	s.updateColoniesStatusLocked()
	s.environment.UpdatedAt = now

	cp := *s.environment
	return &cp, nil
}

func (s *Service) StepSimulation() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.environment.TickCount++
	now := time.Now()

	for _, c := range s.colonies {
		// 1. Natural metabolic energy loss
		c.Energy -= 2.5
		if c.Energy < 0 {
			c.Energy = 0
		}

		// 2. Consume ambient motes if available
		if s.environment.FreeResourceMotes > 0 && c.Energy < c.MaxEnergy {
			c.Energy += 10.0
			s.environment.FreeResourceMotes--
		}

		// 3. Reproduction ("Отщепление частей") when energy reaches threshold
		if c.Energy > (c.MaxEnergy * 0.75) {
			c.Population++
			c.Weights.Generation++
			c.Energy *= 0.6 // division cost

			// Evolution of utility weights (stochastic drift of criteria)
			c.Weights.Alpha += (rand.Float64() - 0.5) * 0.05
			c.Weights.Beta += (rand.Float64() - 0.5) * 0.05
			c.Weights.Gamma += (rand.Float64() - 0.5) * 0.05

			s.log.Info().
				Str("colony", c.ID).
				Int("generation", c.Weights.Generation).
				Int("population", c.Population).
				Msg("Colony reproduced: spawned new node and evolved utility weights")
		}

		c.UpdatedAt = now
	}

	s.updateColoniesStatusLocked()
	s.environment.UpdatedAt = now
}

func (s *Service) updateColoniesStatusLocked() {
	for _, c := range s.colonies {
		if c.Energy <= 50 || c.Entropy >= 80 {
			c.Status = "critical"
		} else if c.Energy < 300 || c.Entropy >= 50 {
			c.Status = "stressed"
		} else if c.Energy > 1000 && c.Entropy < 30 {
			c.Status = "thriving"
		} else {
			c.Status = "stable"
		}
	}
}

func (s *Service) ResetSimulation() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.colonies = make(map[string]*Colony)
	s.seedColonies()
	s.environment.TickCount = 0
	s.environment.SystemEntropy = 18.0
	s.environment.AmbientTemperature = 150.0
	s.environment.FreeResourceMotes = 40
	s.environment.LastEvent = "Simulation reset to baseline state"
	s.environment.UpdatedAt = time.Now()
}

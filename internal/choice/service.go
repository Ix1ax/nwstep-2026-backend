package choice

import (
	"errors"
	"fmt"
	"math"

	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/colony"
)

var (
	ErrDilemmaNotFound = errors.New("dilemma not found")
)

type Service struct {
	colonyService *colony.Service
	dilemmas      map[string]Dilemma
	log           zerolog.Logger
}

func NewService(colonyService *colony.Service, log zerolog.Logger) *Service {
	s := &Service{
		colonyService: colonyService,
		dilemmas:      make(map[string]Dilemma),
		log:           log,
	}

	s.seedDilemmas()
	return s
}

func (s *Service) seedDilemmas() {
	s.dilemmas["solar_flare"] = Dilemma{
		ID:          "solar_flare",
		Title:       "Аномальная солнечная вспышка высокой энергии",
		Description: "Корональный выброс массы несет избыток тепла. Плазме Юпитера и Дюнам Меркурия это выгодно (+энергия), но ледяные гейзеры Энцелада рискуют растаять и погибнуть.",
		Alternatives: []Alternative{
			{
				ID:          "A",
				Title:       "Полный захват энергии гигантами",
				Description: "Юпитер и Меркурий поглощают 100% радиации. Энцелад не защищен.",
				EnergyDeltas: map[string]float64{
					"jupiter":   +800,
					"mercury":   +350,
					"enceladus": -400,
					"triton":    0,
				},
				EntropyDelta: +35.0,
			},
			{
				ID:          "B",
				Title:       "Гравитационный щит Тритона",
				Description: "Тритон и Энцелад сжигают внутренний резерв для рассеивания потока в пустоту космоса.",
				EnergyDeltas: map[string]float64{
					"jupiter":   0,
					"mercury":   0,
					"enceladus": -50,
					"triton":    -80,
				},
				EntropyDelta: +12.0,
			},
			{
				ID:          "C",
				Title:       "Равновесное рассеивание по крио-каналам",
				Description: "Поток дробится через азотную сеть Тритона и охлаждает Энцелад.",
				EnergyDeltas: map[string]float64{
					"jupiter":   +250,
					"mercury":   +150,
					"enceladus": +100,
					"triton":    +50,
				},
				EntropyDelta: -8.0,
			},
		},
	}

	s.dilemmas["beacon_frequency"] = Dilemma{
		ID:          "beacon_frequency",
		Title:       "Выбор единой несущей частоты для гравитационного маяка",
		Description: "Колониям необходимо согласовать единый диапазон обмена телеметрией через пояс астероидов.",
		Alternatives: []Alternative{
			{
				ID:          "A",
				Title:       "Акустический спектр 432 Гц",
				Description: "Идеально резонирует в трубах Тритона, но требует высокой компрессии от плазмы.",
				EnergyDeltas: map[string]float64{
					"jupiter":   -100,
					"mercury":   +50,
					"enceladus": +80,
					"triton":    +350,
				},
				EntropyDelta: +8.0,
			},
			{
				ID:          "B",
				Title:       "Магнитный импульс 12 МГц",
				Description: "Оптимален для дюн Меркурия и плазмы Юпитера.",
				EnergyDeltas: map[string]float64{
					"jupiter":   +300,
					"mercury":   +400,
					"enceladus": -30,
					"triton":    -50,
				},
				EntropyDelta: +15.0,
			},
			{
				ID:          "C",
				Title:       "Широкополосный квантово-гравитонный импульс",
				Description: "Равнодоступный сигнал с минимальным затуханием во всей Солнечной системе.",
				EnergyDeltas: map[string]float64{
					"jupiter":   +120,
					"mercury":   +120,
					"enceladus": +120,
					"triton":    +120,
				},
				EntropyDelta: -14.0,
			},
		},
	}
}

// ExecuteAllocation calculates the result of the slider: "Keep for self" vs "Give to neighbor".
func (s *Service) ExecuteAllocation(req AllocationRequest) (*AllocationResponse, error) {
	if req.SharePercent < 0 {
		req.SharePercent = 0
	}
	if req.SharePercent > 100 {
		req.SharePercent = 100
	}

	source, err := s.colonyService.GetColony(req.SourceColonyID)
	if err != nil {
		return nil, fmt.Errorf("source colony: %w", err)
	}

	target, err := s.colonyService.GetColony(req.TargetColonyID)
	if err != nil {
		return nil, fmt.Errorf("target colony: %w", err)
	}

	// Calculate energy transferred from source to target
	transferFrac := req.SharePercent / 100.0
	maxTransferPool := source.Energy * 0.4 // max 40% of source energy can be redistributed
	transferredEnergy := maxTransferPool * transferFrac

	sourceNewEnergy := source.Energy - transferredEnergy
	targetNewEnergy := target.Energy + transferredEnergy

	// System metrics calculation
	var welfare float64
	var entropy float64
	var msg string

	if req.SharePercent < 25 { // Egoism
		welfare = 52.0 + (req.SharePercent * 0.4)
		entropy = 48.0 - (req.SharePercent * 0.2)
		msg = fmt.Sprintf("⚠️ Эгоизм: %s забирает %.0f%% ресурса себе. %s теряет стабильность.",
			source.Name, 100-req.SharePercent, target.Name)
	} else if req.SharePercent > 75 { // Altruism / Super-symbiosis
		welfare = 80.0 + ((req.SharePercent - 75) * 0.4)
		entropy = 12.0
		msg = fmt.Sprintf("✨ Сверх-симбиоз: %.0f%% потока направлено на подпитку %s. Колония спасена!",
			req.SharePercent, target.Name)
	} else { // Pareto Optimum (around 50%)
		welfare = 88.0
		entropy = 16.0
		msg = fmt.Sprintf("⚡ Парето-баланс: Сбалансированный энергообмен (%.0f%% / %.0f%%). Стабильность экосистемы.",
			100-req.SharePercent, req.SharePercent)
	}

	beamActive := req.SharePercent > 10.0

	return &AllocationResponse{
		SourceColonyID:    source.ID,
		TargetColonyID:    target.ID,
		TransferredEnergy: math.Round(transferredEnergy*10) / 10,
		SourceNewEnergy:   math.Round(sourceNewEnergy*10) / 10,
		TargetNewEnergy:   math.Round(targetNewEnergy*10) / 10,
		SystemWelfare:     math.Round(welfare*10) / 10,
		SystemEntropy:     math.Round(entropy*10) / 10,
		BeamActive:        beamActive,
		Message:           msg,
		RatioTelemetry: fmt.Sprintf("RATIO: Transfer=%.1f J | Source=%.1f J | Target=%.1f J | Entropy=%.1f%%",
			transferredEnergy, sourceNewEnergy, targetNewEnergy, entropy),
	}, nil
}

func (s *Service) GetDilemmas() []Dilemma {
	list := make([]Dilemma, 0, len(s.dilemmas))
	for _, d := range s.dilemmas {
		list = append(list, d)
	}
	return list
}

// ResolveDilemma applies the selected philosophical decision engine to choose the winning alternative.
func (s *Service) ResolveDilemma(req ResolveDilemmaRequest) (*ResolveDilemmaResponse, error) {
	dilemma, ok := s.dilemmas[req.DilemmaID]
	if !ok {
		return nil, ErrDilemmaNotFound
	}

	var bestAlt Alternative
	var bestScore float64 = -math.MaxFloat64
	var explanation string

	switch req.Philosophy {
	case "bentham": // Utilitarianism: Maximize sum of utility
		for _, alt := range dilemma.Alternatives {
			var sum float64
			for _, delta := range alt.EnergyDeltas {
				sum += delta
			}
			if sum > bestScore {
				bestScore = sum
				bestAlt = alt
			}
		}
		explanation = fmt.Sprintf("Утилитаризм (Бентам): максимизирована общая сумма энергии (+%.0f J). Решение принято в пользу большинства.", bestScore)

	case "rawls": // Egalitarianism / Maximin: Maximize the minimum outcome
		for _, alt := range dilemma.Alternatives {
			minDelta := math.MaxFloat64
			for _, delta := range alt.EnergyDeltas {
				if delta < minDelta {
					minDelta = delta
				}
			}
			if minDelta > bestScore {
				bestScore = minDelta
				bestAlt = alt
			}
		}
		explanation = "Эгалитаризм (Ролз): критерий максимина защитил наиболее уязвимую колонию от критического ущерба."

	case "quadratic": // Quadratic Voting / Allocation: sum of sqrt(energy) with sign
		for _, alt := range dilemma.Alternatives {
			var qvSum float64
			for _, delta := range alt.EnergyDeltas {
				sign := 1.0
				if delta < 0 {
					sign = -1.0
				}
				qvSum += sign * math.Sqrt(math.Abs(delta))
			}
			if qvSum > bestScore {
				bestScore = qvSum
				bestAlt = alt
			}
		}
		explanation = "Квадратичный вес: колонии сожгли резерв для усиления волеизъявления. Коалиция средних колоний перевесила гиганта."

	case "entropy": // Minimum Entropy: minimize thermodynamic chaos
		bestScore = math.MaxFloat64
		for _, alt := range dilemma.Alternatives {
			if alt.EntropyDelta < bestScore {
				bestScore = alt.EntropyDelta
				bestAlt = alt
			}
		}
		explanation = fmt.Sprintf("Термодинамический оптимум: минимальный прирост энтропии (%.1f%%). Сохранение порядка в системе.", bestScore)

	default: // Default to Bentham
		bestAlt = dilemma.Alternatives[0]
		explanation = "Применен базовый алгоритм максимизации полезности."
	}

	// Calculate reactions
	reactions := make(map[string]Reaction)
	for colID, delta := range bestAlt.EnergyDeltas {
		var status, text string
		if delta > 100 {
			status = "thriving"
			text = fmt.Sprintf("+%.0f J (Процветание)", delta)
		} else if delta < -50 {
			status = "stressed"
			text = fmt.Sprintf("%.0f J (Критический урон)", delta)
		} else {
			status = "stable"
			text = fmt.Sprintf("%+.0f J (Стабильно)", delta)
		}
		reactions[colID] = Reaction{
			EnergyChange: delta,
			Status:       status,
			ReactionText: text,
		}
	}

	return &ResolveDilemmaResponse{
		DilemmaID:            dilemma.ID,
		Philosophy:           req.Philosophy,
		WinningAlternativeID: bestAlt.ID,
		WinningTitle:         bestAlt.Title,
		SystemWelfare:        82.0,
		SystemEntropy:        math.Max(10.0, 20.0+bestAlt.EntropyDelta),
		ColonyReactions:      reactions,
		Explanation:          explanation,
	}, nil
}

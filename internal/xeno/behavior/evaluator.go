package behavior

import (
	"fmt"
	"math"
	"sort"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
)

// Evaluator реализует логику принятия решений особью согласно разделу 7.3 ТЗ v2
type Evaluator struct {
	params model.Parameters
}

// NewEvaluator создает экземпляр вычислителя поведения
func NewEvaluator(params model.Parameters) *Evaluator {
	return &Evaluator{params: params}
}

// UpdateMemory обновляет динамическую память особи
// Формула: M_next = lambda * M + (1 - lambda) * normalizedNetInput
func (e *Evaluator) UpdateMemory(currentM, lambda, netInflow float64) float64 {
	ref := e.params.ReferenceInflow
	if ref <= 0 {
		ref = 1.0
	}
	normalized := clip(netInflow/ref, -1.0, 1.0)
	nextM := lambda*currentM + (1.0-lambda)*normalized
	return clip(nextM, -1.0, 1.0)
}

// EvaluateDecisions оценивает все 4 альтернативы действия для особи (раздел 7.3 ТЗ v2)
func (e *Evaluator) EvaluateDecisions(
	ind *model.Individual,
	mode model.Mode,
	tick int64,
	channels []*model.Channel,
	neighborSignals map[string]*model.SignalMessage,
	neighborEnergies map[string]float64,
	currentPopulation int,
	hasFreeSpaceForChild bool,
) *model.DecisionTrace {
	// 1. Реактивный режим: фиксированный автомат без памяти
	if mode == model.ModeReactive {
		return e.evaluateReactive(ind, tick, channels, neighborEnergies)
	}

	// 2. Адаптивный и Эволюционный режимы: целевая функция score(a)
	return e.evaluateAdaptive(
		ind,
		mode,
		tick,
		channels,
		neighborSignals,
		neighborEnergies,
		currentPopulation,
		hasFreeSpaceForChild,
	)
}

// evaluateReactive — автомат «стимул — ответ» по текущему запасу (раздел 7.3)
func (e *Evaluator) evaluateReactive(
	ind *model.Individual,
	tick int64,
	channels []*model.Channel,
	neighborEnergies map[string]float64,
) *model.DecisionTrace {
	scores := map[model.ActionType]float64{
		model.ActionStore:    ind.Energy / e.params.EMax,
		model.ActionTransfer: 0.0,
		model.ActionGrow:     0.0,
		model.ActionDivide:   0.0,
	}

	trace := &model.DecisionTrace{
		IndividualID:   ind.ID,
		Mode:           model.ModeReactive,
		Tick:           tick,
		SelectedAction: model.ActionStore,
		Scores:         scores,
		ChosenScore:    scores[model.ActionStore],
		Reasoning:      "Реактивное сохранение энергии",
	}

	// Проверяем возможность передачи по фиксированному порогу
	normE := ind.Energy / e.params.EMax
	threshold := 0.4 // фиксированный порог реактивного режима

	if normE >= threshold {
		// Ищем соседа с наименьшей энергией
		var bestTarget string
		var minNeighborE float64 = math.MaxFloat64

		// Сортируем каналы для детерминизма
		sortedChannels := make([]*model.Channel, 0, len(channels))
		for _, ch := range channels {
			if ch.Enabled && ch.FromID == ind.ID {
				sortedChannels = append(sortedChannels, ch)
			}
		}
		sort.Slice(sortedChannels, func(i, j int) bool { return sortedChannels[i].ID < sortedChannels[j].ID })

		for _, ch := range sortedChannels {
			nE, ok := neighborEnergies[ch.ToID]
			if ok && nE < minNeighborE {
				minNeighborE = nE
				bestTarget = ch.ToID
			}
		}

		if bestTarget != "" && minNeighborE < ind.Energy {
			scores[model.ActionTransfer] = normE
			trace.SelectedAction = model.ActionTransfer
			trace.SelectedTarget = bestTarget
			trace.ChosenScore = normE
			trace.Reasoning = fmt.Sprintf("Реактивная передача ресурса соседу %s (E_neighbor < E_self)", bestTarget)
			return trace
		}
	}

	return trace
}

// evaluateAdaptive — расчет целевой функции score(a) (раздел 7.3 ТЗ v2)
// score(a) = wE * expectedReserve - wD * expectedDeficit + wC * neighborRelief + wR * reproductionResult - wCost * irreversibleCost
func (e *Evaluator) evaluateAdaptive(
	ind *model.Individual,
	mode model.Mode,
	tick int64,
	channels []*model.Channel,
	neighborSignals map[string]*model.SignalMessage,
	neighborEnergies map[string]float64,
	currentPopulation int,
	hasFreeSpaceForChild bool,
) *model.DecisionTrace {
	// Веса генома
	wE := ind.Genome.WeightEnergy
	wD := ind.Genome.WeightDeficit
	wC := ind.Genome.WeightRelief
	wR := ind.Genome.WeightReproduction
	wCost := ind.Genome.WeightCost
	hThreshold := ind.Genome.HThreshold

	// В адаптивном режиме память M модулирует веса безопасности
	if mode != model.ModeReactive {
		// Отрицательная память (дефицит) усиливает приоритет сохранения и штраф за дефицит
		safetyFactor := 1.0 + math.Max(0.0, -ind.Memory)
		wD *= safetyFactor
		wE *= (1.0 + 0.5*math.Max(0.0, -ind.Memory))
		// Помощь соседям и деление снижаются при длительном дефиците
		wC /= safetyFactor
		wR /= safetyFactor
	}

	// Нормируем веса
	sumW := wE + wD + wC + wR + wCost
	if sumW > 0 {
		wE /= sumW
		wD /= sumW
		wC /= sumW
		wR /= sumW
		wCost /= sumW
	}

	calcScore := func(reserve, deficit, relief, repro, cost float64) float64 {
		return wE*reserve - wD*deficit + wC*relief + wR*repro - wCost*cost
	}

	reserveTarget := e.params.ReserveTarget
	eMax := e.params.EMax
	twoTicksMaint := 2.0 * e.params.MaintenancePowerPm * e.params.Dt

	// 1. Оценка действия STORE
	resStore := clip(ind.Energy/reserveTarget, 0.0, 1.0)
	defStore := clip(math.Max(0.0, twoTicksMaint-ind.Energy)/twoTicksMaint, 0.0, 1.0)
	scoreStore := calcScore(resStore, defStore, 0.0, 0.0, 0.0)

	scores := map[model.ActionType]float64{
		model.ActionStore:    round(scoreStore, 6),
		model.ActionTransfer: -1.0,
		model.ActionGrow:     -1.0,
		model.ActionDivide:   -1.0,
	}

	bestAction := model.ActionStore
	bestTarget := ""
	bestScore := scoreStore

	// 2. Оценка действия TRANSFER
	sortedChannels := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch.Enabled && ch.FromID == ind.ID {
			sortedChannels = append(sortedChannels, ch)
		}
	}
	sort.Slice(sortedChannels, func(i, j int) bool { return sortedChannels[i].ID < sortedChannels[j].ID })

	availableBudget := math.Max(0.0, ind.Energy-twoTicksMaint)
	var bestTransferScore float64 = -1.0
	var bestTransferTarget string

	for _, ch := range sortedChannels {
		if availableBudget <= 0 {
			continue
		}

		// Доставка ресурса соседу
		eSent := math.Min(availableBudget, ch.MaxPower*e.params.Dt)
		if eSent <= 0 {
			continue
		}
		eDelivered := eSent * (1.0 - ch.Loss)

		// Оценка потребности соседа по сигналу или известной энергии
		var neighborDeficit float64
		if sig, ok := neighborSignals[ch.ToID]; ok {
			if sig.IsStarving {
				neighborDeficit = 1.0
			} else {
				neighborDeficit = clip(1.0-sig.NormalizedE, 0.0, 1.0)
			}
		} else if nE, ok := neighborEnergies[ch.ToID]; ok {
			neighborDeficit = clip(math.Max(0.0, reserveTarget-nE)/reserveTarget, 0.0, 1.0)
		}

		relief := clip((eDelivered/reserveTarget)*neighborDeficit, 0.0, 1.0)
		eAfter := math.Max(0.0, ind.Energy-eSent)
		resTransfer := clip(eAfter/reserveTarget, 0.0, 1.0)
		defTransfer := clip(math.Max(0.0, twoTicksMaint-eAfter)/twoTicksMaint, 0.0, 1.0)
		costTransfer := clip((eSent*ch.Loss)/eMax, 0.0, 1.0)

		scoreT := calcScore(resTransfer, defTransfer, relief, 0.0, costTransfer)
		if scoreT > bestTransferScore {
			bestTransferScore = scoreT
			bestTransferTarget = ch.ToID
		}
	}

	if bestTransferTarget != "" {
		scores[model.ActionTransfer] = round(bestTransferScore, 6)
		if (bestTransferScore-scoreStore >= hThreshold) && bestTransferScore > bestScore {
			bestScore = bestTransferScore
			bestAction = model.ActionTransfer
			bestTarget = bestTransferTarget
		}
	}

	// 3. Оценка действия GROW
	growBudget := math.Min(availableBudget, e.params.PGrowthMax*e.params.Dt)
	if growBudget > 0 && ind.Biomass < e.params.BSplit {
		eAfterGrow := math.Max(0.0, ind.Energy-growBudget)
		resGrow := clip(eAfterGrow/reserveTarget, 0.0, 1.0)
		defGrow := clip(math.Max(0.0, twoTicksMaint-eAfterGrow)/twoTicksMaint, 0.0, 1.0)

		deltaB := (growBudget * e.params.EtaGrowth) / e.params.KB
		reproGrow := clip((ind.Biomass+deltaB)/e.params.BSplit, 0.0, 1.0)
		costGrow := clip((growBudget*(1.0-e.params.EtaGrowth))/eMax, 0.0, 1.0)

		scoreGrow := calcScore(resGrow, defGrow, 0.0, reproGrow, costGrow)
		scores[model.ActionGrow] = round(scoreGrow, 6)

		if (scoreGrow-scoreStore >= hThreshold) && scoreGrow > bestScore {
			bestScore = scoreGrow
			bestAction = model.ActionGrow
			bestTarget = ""
		}
	}

	// 4. Оценка действия DIVIDE
	reqEnergy := e.params.ChildInitialEnergy + e.params.DivisionCost + twoTicksMaint
	dividePossible := ind.Biomass >= e.params.BSplit &&
		ind.Energy >= reqEnergy &&
		(tick-ind.LastDivisionTick >= e.params.DivisionCooldown) &&
		currentPopulation < e.params.MaxPopulation &&
		hasFreeSpaceForChild

	if dividePossible {
		eAfterDivide := math.Max(0.0, ind.Energy-e.params.ChildInitialEnergy-e.params.DivisionCost)
		resDivide := clip(eAfterDivide/reserveTarget, 0.0, 1.0)
		defDivide := clip(math.Max(0.0, twoTicksMaint-eAfterDivide)/twoTicksMaint, 0.0, 1.0)
		reproDivide := 1.0
		costDivide := clip(e.params.DivisionCost/eMax, 0.0, 1.0)

		scoreDivide := calcScore(resDivide, defDivide, 0.0, reproDivide, costDivide)
		scores[model.ActionDivide] = round(scoreDivide, 6)

		if (scoreDivide-scoreStore >= hThreshold) && scoreDivide > bestScore {
			bestScore = scoreDivide
			bestAction = model.ActionDivide
			bestTarget = ""
		}
	}

	trace := &model.DecisionTrace{
		IndividualID:   ind.ID,
		Mode:           mode,
		Tick:           tick,
		SelectedAction: bestAction,
		SelectedTarget: bestTarget,
		Scores:         scores,
		ChosenScore:    round(bestScore, 6),
	}

	switch bestAction {
	case model.ActionStore:
		trace.Reasoning = "Сохранение ресурса: ни одна альтернатива не превзошла порог h"
	case model.ActionTransfer:
		trace.Reasoning = fmt.Sprintf("Передача ресурса соседу %s по максимуму целевой функции", bestTarget)
	case model.ActionGrow:
		trace.Reasoning = "Наращивание структуры биомассы для подготовки к делению"
	case model.ActionDivide:
		trace.Reasoning = "Воспроизводство: биомасса и энергия достаточны для деления особи"
	}

	return trace
}

func clip(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func round(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

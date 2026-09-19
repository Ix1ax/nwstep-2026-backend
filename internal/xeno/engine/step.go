package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/behavior"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/environment"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/evolution"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/prng"
)

// Engine управляет детерминированным шагом симуляции мира
type Engine struct {
	params    model.Parameters
	evaluator *behavior.Evaluator
	mutator   *evolution.Mutator
}

// NewEngine создает новый экземпляр движка
func NewEngine(params model.Parameters) *Engine {
	return &Engine{
		params:    params,
		evaluator: behavior.NewEvaluator(params),
		mutator:   evolution.NewMutator(params),
	}
}

// StepState содержит изменяемое состояние симуляции для такта
type StepState struct {
	Tick              int64
	Revision          int64
	Mode              model.Mode
	World             model.World
	Env               environment.EnvironmentModule
	Individuals       map[string]*model.Individual
	Colonies          map[string]*model.Colony
	Channels          map[string]*model.Channel
	InTransit         []*model.ResourcePacket
	Signals           []*model.SignalMessage
	Balance           model.EnergyBalance
	Streams           *prng.Streams
	BirthsTotal       int
	ColonySplitsTotal int
	DeathsTotal       int
	InitialCount      int
	Interventions     []*model.Intervention
	NextColonyNum     int
	UsefulGrowth      float64
	DeliverySum       float64
	DeliveryCount     int
	ResponseTick      int64
	ResponseBaseline  map[string]model.ActionType
	ResponseLatency   float64
	ResponseMeasured  bool
}

func getSortedIndividualIDs(m map[string]*model.Individual) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func getSortedColonyIDs(m map[string]*model.Colony) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func getSortedChannelIDs(m map[string]*model.Channel) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Step выполняет ровно один детерминированный такт симуляции строго по 12 шагам ТЗ v2
func (eng *Engine) Step(st *StepState) (*model.StateSnapshot, *model.MetricsSnapshot, error) {
	t := st.Tick + 1
	dt := eng.params.Dt

	// ─────────────────────────────────────────────────────────────
	// Шаг 1: Применить вмешательства с tick == t в порядке sequence
	// ─────────────────────────────────────────────────────────────
	currentInterventions := make([]*model.Intervention, 0)
	for _, it := range st.Interventions {
		if it.Tick == t {
			currentInterventions = append(currentInterventions, it)
		}
	}
	sort.Slice(currentInterventions, func(i, j int) bool {
		return currentInterventions[i].Sequence < currentInterventions[j].Sequence
	})

	for _, it := range currentInterventions {
		switch it.Type {
		case "add_inoculum":
			eng.addColony(st, it, t)
		case "set_mode":
			st.Mode = model.Mode(it.TargetID)
		case "set_channel":
			eng.editChannel(st, it)

		case "set_flow", "set_noise", "impulse", "perturbation", "depletion":
			// Environment settings are reconstructed below.

		case "toggle_mutations":
			if it.Value > 0.5 {
				st.Mode = model.ModeEvolutionary
			} else {
				st.Mode = model.ModeAdaptive
			}
		}
	}

	st.Env = environment.NewEnvironmentModule(st.World)
	for _, it := range st.Interventions {
		if it.Tick <= t && (it.Duration == 0 || t < it.Tick+it.Duration) {
			st.Env.ApplyIntervention(it)
		}
	}
	if len(currentInterventions) > 0 {
		st.ResponseTick = t
		st.ResponseMeasured = false
		st.ResponseLatency = 0
		st.ResponseBaseline = map[string]model.ActionType{}
		for id, ind := range st.Individuals {
			if ind.LastDecision != nil {
				st.ResponseBaseline[id] = ind.LastDecision.SelectedAction
			}
		}
	}
	netInputs := map[string]float64{}

	// ─────────────────────────────────────────────────────────────
	// Шаг 2: Доставить сигналы и пакеты с deliveryTick <= t
	// ─────────────────────────────────────────────────────────────
	deliveredPackets := make([]*model.ResourcePacket, 0)
	remainingPackets := make([]*model.ResourcePacket, 0)

	for _, pkt := range st.InTransit {
		if pkt.DeliveryTick <= t {
			deliveredPackets = append(deliveredPackets, pkt)
		} else {
			remainingPackets = append(remainingPackets, pkt)
		}
	}
	st.InTransit = remainingPackets

	for _, pkt := range deliveredPackets {
		st.Balance.InTransit -= pkt.NetEnergy
		st.DeliverySum += float64(t - pkt.EmittedTick)
		st.DeliveryCount++
		receiver, ok := st.Individuals[pkt.ReceiverID]
		if ok && receiver.Alive {
			receiver.Energy += pkt.NetEnergy
			if receiver.Energy > eng.params.EMax {
				overflow := receiver.Energy - eng.params.EMax
				receiver.Energy = eng.params.EMax
				st.Balance.Overflow += overflow
			}
		} else {
			// Получатель погиб в пути: полезная энергия уходит в рассеяние при смерти
			st.Balance.DeathDissipation += pkt.NetEnergy
		}
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 3: Начислить приток внешней энергии среды
	// ─────────────────────────────────────────────────────────────
	sortedIndIDs := getSortedIndividualIDs(st.Individuals)
	var stepExternalInput float64

	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}

		uNoise := st.Streams.Environment.NextFloat64()
		sample := *ind
		sample.Age = t
		input := st.Env.ResourceInput(&sample, dt, uNoise)
		netInputs[id] = input - st.Env.MaintenanceCost(ind, dt)
		stepExternalInput += input
		st.Balance.ExternalInput += input

		ind.Energy += input
		if ind.Energy > eng.params.EMax {
			overflow := ind.Energy - eng.params.EMax
			ind.Energy = eng.params.EMax
			st.Balance.Overflow += overflow
		}
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 4: Оплатить поддержание, определить смерти
	// ─────────────────────────────────────────────────────────────
	var stepUsefulMaintenance float64

	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}
		ind.Age++

		requiredMaint := st.Env.MaintenanceCost(ind, dt)

		if ind.Energy >= requiredMaint {
			ind.Energy -= requiredMaint
			ind.StarvationTicks = 0
			st.Balance.Maintenance += requiredMaint
			stepUsefulMaintenance += requiredMaint
		} else {
			// Дефицит энергии
			deficitSpent := ind.Energy
			ind.Energy = 0
			ind.StarvationTicks++
			st.Balance.Maintenance += deficitSpent
			stepUsefulMaintenance += deficitSpent

			if ind.StarvationTicks >= eng.params.StarvationLimit {
				// Особь погибает
				ind.Alive = false
				ind.DeathTick = &t
				ind.DeathReason = "starvation"
				st.DeathsTotal++

				// Остаток ресурсов переходит в рассеяние при смерти
				dissipated := ind.Energy + eng.params.KB*ind.Biomass
				st.Balance.DeathDissipation += dissipated
				ind.Energy = 0
				ind.Biomass = 0
			}
		}
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 5: Обновить память живых особей
	// ─────────────────────────────────────────────────────────────
	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}
		netInflow := netInputs[id]
		ind.Memory = eng.evaluator.UpdateMemory(ind.Memory, ind.Genome.Lambda, netInflow)
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 6: Зафиксировать снимок локальных наблюдений (без всезнания)
	// ─────────────────────────────────────────────────────────────
	neighborEnergies := make(map[string]float64)
	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if ind.Alive {
			neighborEnergies[id] = ind.Energy
		}
	}

	latestSignals := make(map[string]map[string]*model.SignalMessage) // receiverID -> senderID -> sig
	for _, sig := range st.Signals {
		if sig.DeliveryTick <= t {
			if _, ok := latestSignals[sig.ReceiverID]; !ok {
				latestSignals[sig.ReceiverID] = make(map[string]*model.SignalMessage)
			}
			latestSignals[sig.ReceiverID][sig.SenderID] = sig
		}
	}

	sortedChannelIDs := getSortedChannelIDs(st.Channels)
	allChannels := make([]*model.Channel, 0, len(sortedChannelIDs))
	for _, id := range sortedChannelIDs {
		allChannels = append(allChannels, st.Channels[id])
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 7: Рассчитать решения особей в стабильном порядке ID
	// ─────────────────────────────────────────────────────────────
	decisions := make(map[string]*model.DecisionTrace)
	actionCounts := map[model.ActionType]int{
		model.ActionStore:    0,
		model.ActionTransfer: 0,
		model.ActionGrow:     0,
		model.ActionDivide:   0,
	}

	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}
		signalsForInd := latestSignals[id]

		trace := eng.evaluator.EvaluateDecisions(
			ind,
			st.Mode,
			t,
			allChannels,
			signalsForInd,
			neighborEnergies,
			len(sortedIndIDs),
			true,
		)
		decisions[id] = trace
		ind.LastDecision = trace
		actionCounts[trace.SelectedAction]++
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 8: Исполнить передачи и рост, проверить бюджеты
	// ─────────────────────────────────────────────────────────────
	var stepUsefulGrowth float64
	twoTicksMaint := 2.0 * eng.params.MaintenancePowerPm * dt

	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}
		trace, hasTrace := decisions[id]
		if !hasTrace {
			continue
		}

		switch trace.SelectedAction {
		case model.ActionTransfer:
			targetID := trace.SelectedTarget
			var ch *model.Channel
			for _, c := range allChannels {
				if c.FromID == ind.ID && c.ToID == targetID && c.Enabled {
					ch = c
					break
				}
			}
			if ch == nil {
				continue
			}

			availableBudget := math.Max(0.0, ind.Energy-twoTicksMaint)
			eSent := math.Min(availableBudget, ch.MaxPower*dt)

			if eSent > 0 {
				ind.Energy -= eSent
				lossEnergy := eSent * ch.Loss
				netEnergy := eSent - lossEnergy

				st.Balance.ChannelLoss += lossEnergy
				st.Balance.InTransit += netEnergy

				st.InTransit = append(st.InTransit, &model.ResourcePacket{
					ID:           fmt.Sprintf("pkt-%s-%s-%d", ind.ID, targetID, t),
					ChannelID:    ch.ID,
					SenderID:     ind.ID,
					ReceiverID:   targetID,
					SentEnergy:   eSent,
					NetEnergy:    netEnergy,
					LossEnergy:   lossEnergy,
					DeliveryTick: t + int64(ch.DelayTicks),
					EmittedTick:  t,
				})
			}

		case model.ActionGrow:
			availableBudget := math.Max(0.0, ind.Energy-twoTicksMaint)
			growBudget := math.Min(availableBudget, eng.params.PGrowthMax*dt)

			if growBudget > 0 {
				ind.Energy -= growBudget
				deltaB := (growBudget * eng.params.EtaGrowth) / eng.params.KB
				ind.Biomass += deltaB

				conversionLoss := growBudget * (1.0 - eng.params.EtaGrowth)
				st.Balance.ConversionLoss += conversionLoss

				stepUsefulGrowth += deltaB * eng.params.KB
			}
		}
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 9: Исполнить деления особей в порядке ID родителей
	// ─────────────────────────────────────────────────────────────
	for _, id := range sortedIndIDs {
		ind := st.Individuals[id]
		if !ind.Alive {
			continue
		}
		trace, hasTrace := decisions[id]
		if !hasTrace || trace.SelectedAction != model.ActionDivide {
			continue
		}

		if ind.Biomass < eng.params.BSplit {
			continue
		}
		reqEnergy := eng.params.ChildInitialEnergy + eng.params.DivisionCost + twoTicksMaint
		if ind.Energy < reqEnergy {
			continue
		}
		if len(st.Individuals) >= eng.params.MaxPopulation {
			continue
		}

		// Ищем свободное место для потомка на сфере
		existingInds := make([]*model.Colony, 0)
		_ = existingInds
		existingList := make([]*model.Individual, 0, len(sortedIndIDs))
		for _, exID := range sortedIndIDs {
			existingList = append(existingList, st.Individuals[exID])
		}

		candLat, candLng, ok := eng.mutator.FindChildPosition(ind.Lat, ind.Lng, existingList, st.Streams)
		if !ok {
			continue
		}

		// Выполняем деление особи
		ind.Biomass -= eng.params.BSplit
		parentBiomassRetained := eng.params.BSplit / 2.0
		childBiomass := eng.params.BSplit / 2.0
		ind.Biomass += parentBiomassRetained

		ind.Energy -= (eng.params.ChildInitialEnergy + eng.params.DivisionCost)
		st.Balance.DivisionCost += eng.params.DivisionCost

		ind.LastDivisionTick = t
		st.BirthsTotal++

		childGenome := eng.mutator.InheritGenome(ind.Genome, st.Mode, st.Streams)
		childID := fmt.Sprintf("ind-%04d", len(st.Individuals)+1)

		child := &model.Individual{
			ID:               childID,
			WorldID:          ind.WorldID,
			ColonyID:         ind.ColonyID,
			ParentID:         &ind.ID,
			Lat:              candLat,
			Lng:              candLng,
			Energy:           eng.params.ChildInitialEnergy,
			Biomass:          childBiomass,
			Memory:           0.0,
			Genome:           childGenome,
			Alive:            true,
			Age:              0,
			Generation:       ind.Generation + 1,
			StarvationTicks:  0,
			BirthTick:        t,
			LastDivisionTick: t,
		}
		st.Individuals[childID] = child

		// Добавляем особь в список колонии
		if col, ok := st.Colonies[ind.ColonyID]; ok {
			col.IndividualIDs = append(col.IndividualIDs, childID)
		}

		// Создаем двусторонние каналы связи между родителем и потомком
		dist, loss, delay := st.Env.ChannelProperties(ind, child)
		chFwd := &model.Channel{
			ID:          fmt.Sprintf("ch-%s-%s", ind.ID, childID),
			FromID:      ind.ID,
			ToID:        childID,
			Distance:    dist,
			Conductance: eng.params.ConductanceG,
			MaxPower:    eng.params.MaxChannelPower,
			Loss:        loss,
			DelayTicks:  delay,
			Enabled:     true,
		}
		chRev := &model.Channel{
			ID:          fmt.Sprintf("ch-%s-%s", childID, ind.ID),
			FromID:      childID,
			ToID:        ind.ID,
			Distance:    dist,
			Conductance: eng.params.ConductanceG,
			MaxPower:    eng.params.MaxChannelPower,
			Loss:        loss,
			DelayTicks:  delay,
			Enabled:     true,
		}
		st.Channels[chFwd.ID] = chFwd
		st.Channels[chRev.ID] = chRev
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 10: Отправить сигналы на тактах, кратных signalPeriod
	// ─────────────────────────────────────────────────────────────
	if t%eng.params.SignalPeriod == 0 {
		sortedChannels := make([]*model.Channel, 0, len(st.Channels))
		for _, ch := range st.Channels {
			sortedChannels = append(sortedChannels, ch)
		}
		sort.Slice(sortedChannels, func(i, j int) bool {
			return sortedChannels[i].ID < sortedChannels[j].ID
		})

		for _, ch := range sortedChannels {
			if !ch.Enabled {
				continue
			}
			sender, ok := st.Individuals[ch.FromID]
			if !ok || !sender.Alive {
				continue
			}

			cost := eng.params.SignalCostPerChannel
			if sender.Energy >= cost {
				sender.Energy -= cost
				st.Balance.Signaling += cost

				st.Signals = append(st.Signals, &model.SignalMessage{
					ID:           fmt.Sprintf("sig-%s-%s-%d", sender.ID, ch.ToID, t),
					SenderID:     sender.ID,
					ReceiverID:   ch.ToID,
					NormalizedE:  sender.Energy / eng.params.EMax,
					Value:        sender.Energy,
					IsStarving:   sender.StarvationTicks > 0,
					EmittedTick:  t,
					DeliveryTick: t + int64(ch.DelayTicks),
				})
			}
		}
	}

	// Очищаем старые доставленные сигналы
	activeSignals := make([]*model.SignalMessage, 0)
	for _, sig := range st.Signals {
		if (t - sig.DeliveryTick) <= eng.params.SignalTTL {
			activeSignals = append(activeSignals, sig)
		}
	}
	st.Signals = activeSignals

	// ─────────────────────────────────────────────────────────────
	// Шаг 11: Пересчитать состав колоний и проверить образование дочерних
	// ─────────────────────────────────────────────────────────────
	sortedColonyIDs := getSortedColonyIDs(st.Colonies)
	for _, cID := range sortedColonyIDs {
		col := st.Colonies[cID]

		// Проверяем почкование
		if len(st.Colonies) < st.World.Model.MaxColonies {
			if daughter, remainingIDs, splitOk := eng.mutator.CheckColonyBudding(col, st.Individuals, t, st.NextColonyNum); splitOk {
				st.NextColonyNum++
				st.ColonySplitsTotal++
				col.IndividualIDs = remainingIDs
				st.Colonies[daughter.ID] = daughter

				// Переназначаем colonyId у отделившихся особей
				for _, dID := range daughter.IndividualIDs {
					if ind, ok := st.Individuals[dID]; ok {
						ind.ColonyID = daughter.ID
					}
				}
			}
		}

		// Пересчет метрик колонии
		var cEnergy, cBiomass float64
		cDecisions := map[model.ActionType]int{
			model.ActionStore:    0,
			model.ActionTransfer: 0,
			model.ActionGrow:     0,
			model.ActionDivide:   0,
		}
		cLiving := 0

		for _, indID := range col.IndividualIDs {
			ind, ok := st.Individuals[indID]
			if ok && ind.Alive {
				cLiving++
				cEnergy += ind.Energy
				cBiomass += ind.Biomass
				if ind.LastDecision != nil {
					cDecisions[ind.LastDecision.SelectedAction]++
				}
			}
		}

		col.Metrics = model.ColonyMetrics{
			Population:           cLiving,
			TotalEnergy:          cEnergy,
			TotalBiomass:         cBiomass,
			DecisionDistribution: cDecisions,
		}
	}

	// ─────────────────────────────────────────────────────────────
	// Шаг 12: Баланс энергии, расчет метрик и SHA-256 чексумма
	// ─────────────────────────────────────────────────────────────
	var currentStored float64
	var livingCount int
	var initialLivingCount int
	var sumWelfare float64

	for _, id := range getSortedIndividualIDs(st.Individuals) {
		ind := st.Individuals[id]
		if ind.Alive {
			livingCount++
			currentStored += (ind.Energy + eng.params.KB*ind.Biomass)
			if ind.ParentID == nil {
				initialLivingCount++
			}
			sumWelfare += math.Min(ind.Energy/eng.params.ReserveTarget, 1.0)
		}
	}
	st.Balance.CurrentStored = currentStored

	// Проверка уравнения баланса энергии (раздел 11 ТЗ v2)
	totalExpenditures := st.Balance.CurrentStored + st.Balance.InTransit +
		st.Balance.Maintenance + st.Balance.Signaling + st.Balance.ChannelLoss +
		st.Balance.ConversionLoss + st.Balance.DivisionCost +
		st.Balance.Overflow + st.Balance.DeathDissipation

	balanceResidual := math.Abs((st.Balance.InitialStored + st.Balance.Inoculated + st.Balance.ExternalInput) - totalExpenditures)

	// Информационная энтропия действий: H = -sum p(a) * log2(p(a)) (0..2 бита)
	var decisionEntropy float64
	decisionCount := 0
	for _, n := range actionCounts {
		decisionCount += n
	}
	if decisionCount > 0 {
		actionOrder := []model.ActionType{
			model.ActionStore,
			model.ActionTransfer,
			model.ActionGrow,
			model.ActionDivide,
		}
		for _, act := range actionOrder {
			count := actionCounts[act]
			if count > 0 {
				p := float64(count) / float64(decisionCount)
				decisionEntropy -= p * math.Log2(p)
			}
		}
	}

	survivalRate := 0.0
	if st.InitialCount > 0 {
		survivalRate = (float64(initialLivingCount) / float64(st.InitialCount)) * 100.0
	}

	meanWelfare := 0.0
	if livingCount > 0 {
		meanWelfare = (sumWelfare / float64(livingCount)) * 100.0
	}

	inputPower := stepExternalInput / dt
	usefulPower := (stepUsefulMaintenance + stepUsefulGrowth) / dt

	var efficiency float64
	st.UsefulGrowth += stepUsefulGrowth
	effDenom := st.Balance.ExternalInput + st.Balance.InitialStored + st.Balance.Inoculated
	if effDenom > 0 {
		efficiency = math.Min(100.0, (st.Balance.Maintenance+st.UsefulGrowth)/effDenom*100.0)
	}

	// Считаем активные колонии (где есть хотя бы 1 живая особь)
	activeColoniesCount := 0
	for _, col := range st.Colonies {
		if col.Metrics.Population > 0 {
			activeColoniesCount++
		}
	}

	deliveryLatency := 0.0
	if st.DeliveryCount > 0 {
		deliveryLatency = st.DeliverySum / float64(st.DeliveryCount)
	}
	if !st.ResponseMeasured && st.ResponseTick > 0 {
		for id, old := range st.ResponseBaseline {
			if ind := st.Individuals[id]; ind != nil && ind.Alive && ind.LastDecision != nil && ind.LastDecision.SelectedAction != old {
				st.ResponseMeasured = true
				st.ResponseLatency = float64(t - st.ResponseTick)
				break
			}
		}
	}
	metrics := &model.MetricsSnapshot{
		Tick:              t,
		TimeTU:            float64(t) * dt,
		Population:        livingCount,
		ActiveColonies:    activeColoniesCount,
		SurvivalRate:      survivalRate,
		InputPower:        inputPower,
		UsefulPower:       usefulPower,
		Efficiency:        efficiency,
		DecisionEntropy:   decisionEntropy,
		DeliveryLatency:   deliveryLatency,
		DeliveryMeasured:  st.DeliveryCount > 0,
		ResponseMeasured:  st.ResponseMeasured,
		ResponseLatency:   st.ResponseLatency,
		BirthsTotal:       st.BirthsTotal,
		ColonySplitsTotal: st.ColonySplitsTotal,
		DeathsTotal:       st.DeathsTotal,
		MeanWelfare:       meanWelfare,
		BalanceResidual:   balanceResidual,
	}

	// Вычисляем SHA-256 чексумму канонического состояния
	st.Revision++
	st.Tick = t
	checksum := computeStateChecksum(st)

	// Собираем снимок для возврата
	snapshot := &model.StateSnapshot{
		Mode: st.Mode, Flow: st.Env.GetFlowMultiplier(), Noise: st.Env.GetNoiseMultiplier(), Interventions: st.Interventions,
		Tick:        t,
		Revision:    st.Revision,
		Checksum:    checksum,
		Status:      model.StatusRunning,
		World:       st.World,
		Individuals: exportIndividuals(st.Individuals),
		Colonies:    exportColonies(st.Colonies),
		Channels:    exportChannels(st.Channels),
		InTransit:   exportPackets(st.InTransit),
		Signals:     exportSignals(st.Signals),
		Metrics:     *metrics,
		Balance:     st.Balance,
	}

	return snapshot, metrics, nil
}

func computeStateChecksum(st *StepState) string {
	// JSON sorts map keys and preserves full float precision. Exclude only UI status/revision.
	data, _ := json.Marshal(struct {
		Tick                         int64
		Mode                         model.Mode
		World                        model.World
		Individuals                  map[string]*model.Individual
		Colonies                     map[string]*model.Colony
		Channels                     map[string]*model.Channel
		Packets                      []*model.ResourcePacket
		Signals                      []*model.SignalMessage
		Balance                      model.EnergyBalance
		Random                       [3]uint64
		Flow, Noise, Growth          float64
		Births, Deaths, Splits, Next int
	}{st.Tick, st.Mode, st.World, st.Individuals, st.Colonies, st.Channels, st.InTransit, st.Signals, st.Balance, [3]uint64{st.Streams.Environment.State(), st.Streams.Mutations.State(), st.Streams.Placement.State()}, st.Env.GetFlowMultiplier(), st.Env.GetNoiseMultiplier(), st.UsefulGrowth, st.BirthsTotal, st.DeathsTotal, st.ColonySplitsTotal, st.NextColonyNum})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func exportIndividuals(m map[string]*model.Individual) []model.Individual {
	res := make([]model.Individual, 0, len(m))
	for _, v := range m {
		res = append(res, *v)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res
}

func exportColonies(m map[string]*model.Colony) []model.Colony {
	res := make([]model.Colony, 0, len(m))
	for _, v := range m {
		res = append(res, *v)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res
}

func exportChannels(m map[string]*model.Channel) []model.Channel {
	res := make([]model.Channel, 0, len(m))
	for _, v := range m {
		res = append(res, *v)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res
}

func exportPackets(list []*model.ResourcePacket) []model.ResourcePacket {
	res := make([]model.ResourcePacket, 0, len(list))
	for _, v := range list {
		res = append(res, *v)
	}
	return res
}

func exportSignals(list []*model.SignalMessage) []model.SignalMessage {
	res := make([]model.SignalMessage, 0, len(list))
	for _, v := range list {
		res = append(res, *v)
	}
	return res
}

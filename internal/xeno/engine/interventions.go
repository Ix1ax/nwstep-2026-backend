package engine

import (
	"fmt"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"math"
)

func (eng *Engine) addColony(st *StepState, it *model.Intervention, t int64) {
	n := int(it.Params["count"])
	if n == 0 {
		n = 6
	}
	if len(st.Individuals)+n > eng.params.MaxPopulation || len(st.Colonies) >= st.World.Model.MaxColonies {
		return
	}
	id := fmt.Sprintf("colony-%02d", st.NextColonyNum)
	st.NextColonyNum++
	name := it.Name
	if name == "" {
		name = "Новое сообщество"
	}
	color := it.Color
	if color == "" {
		color = "#81d6b9"
	}
	col := &model.Colony{ID: id, WorldID: st.World.ID, Name: name, Color: color, FormedAtTick: t, IndividualIDs: []string{}}
	st.Colonies[id] = col
	genome := model.DefaultGenome()
	if it.Genome != nil {
		genome = *it.Genome
	}
	energy := it.Value
	if energy <= 0 {
		energy = 35
	}
	biomass := it.Params["biomass"]
	if biomass <= 0 {
		biomass = 5
	}
	spread := it.Params["spread"]
	if spread <= 0 {
		spread = 2
	}
	var previous *model.Individual
	var first *model.Individual
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		lat := math.Max(-89.9, math.Min(89.9, it.Params["lat"]+spread*math.Sin(angle)))
		lng := math.Mod(it.Params["lng"]+spread*math.Cos(angle)+540, 360) - 180
		indID := fmt.Sprintf("ind-%04d", len(st.Individuals)+1)
		ind := &model.Individual{ID: indID, WorldID: st.World.ID, ColonyID: id, Lat: lat, Lng: lng, Energy: energy, Biomass: biomass, Genome: genome, Alive: true, Generation: 1, BirthTick: t, LastDivisionTick: t}
		st.Individuals[indID] = ind
		col.IndividualIDs = append(col.IndividualIDs, indID)
		st.Balance.Inoculated += energy + eng.params.KB*biomass
		if previous != nil {
			eng.connect(st, previous, ind, it.Params["power"])
		}
		if first == nil {
			first = ind
		}
		previous = ind
	}
	if n > 2 {
		eng.connect(st, previous, first, it.Params["power"])
	}
}
func (eng *Engine) connect(st *StepState, a, b *model.Individual, power float64) {
	if power <= 0 {
		power = eng.params.MaxChannelPower
	}
	d, l, delay := st.Env.ChannelProperties(a, b)
	for _, pair := range [][2]string{{a.ID, b.ID}, {b.ID, a.ID}} {
		id := "ch-" + pair[0] + "-" + pair[1]
		st.Channels[id] = &model.Channel{ID: id, FromID: pair[0], ToID: pair[1], Distance: d, Loss: l, DelayTicks: delay, MaxPower: power, Conductance: eng.params.ConductanceG, Enabled: true}
	}
}
func (eng *Engine) editChannel(st *StepState, it *model.Intervention) {
	ch := st.Channels[it.TargetID]
	if ch == nil {
		return
	}
	ch.Enabled = it.Value > 0.5
	if v, ok := it.Params["power"]; ok {
		ch.MaxPower = v
	}
	if v, ok := it.Params["loss"]; ok {
		ch.Loss = v
	}
	if v, ok := it.Params["delay"]; ok {
		ch.DelayTicks = int(v)
	}
}

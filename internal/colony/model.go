package colony

import "time"

// Colony represents an autonomous non-biological life form from the hackathon legend.
type Colony struct {
	ID            string           `db:"id" json:"id"`
	Name          string           `db:"name" json:"name"`
	CelestialBody string           `db:"celestial_body" json:"celestial_body"`
	LifeFormType  string           `db:"life_form_type" json:"life_form_type"` // plasma, cryo_geyser, ferromagnetic_dune, acoustic_network
	SignalType    string           `db:"signal_type" json:"signal_type"`       // radio_ionization, pressure_plume, magnetic_flux, acoustic_resonance
	SignalValue   string           `db:"signal_value" json:"signal_value"`     // e.g. "Ne = 1.4e18 cm⁻³", "P = 840 kPa", "B = 142 mT", "f = 432 Hz"
	Energy        float64          `db:"energy" json:"energy"`
	MaxEnergy     float64          `db:"max_energy" json:"max_energy"`
	Entropy       float64          `db:"entropy" json:"entropy"` // 0.0 to 100.0%
	Temperature   float64          `db:"temperature" json:"temperature"`
	OptimalTemp   float64          `db:"optimal_temp" json:"optimal_temp"`
	Population    int              `db:"population" json:"population"` // Node/cluster count ("отщепление частей")
	Weights       EvolutionWeights `db:"weights" json:"weights"`
	Status        string           `db:"status" json:"status"` // thriving, stable, stressed, critical
	PartnerID     string           `db:"partner_id" json:"partner_id"`
	UpdatedAt     time.Time        `db:"updated_at" json:"updated_at"`
}

// EvolutionWeights represents evolving criteria of optimal good ("критерии поиска оптимального блага могут эволюционировать").
type EvolutionWeights struct {
	Alpha      float64 `json:"alpha"`      // Valuation of raw energy
	Beta       float64 `json:"beta"`       // Temperature sensitivity penalty
	Gamma      float64 `json:"gamma"`      // Entropy aversion penalty
	Generation int     `json:"generation"` // Number of division cycles
}

// EnvironmentState represents the planetary sandbox conditions manipulated by the researcher.
type EnvironmentState struct {
	SolarRadiation     float64   `json:"solar_radiation"`     // 0-100%
	AmbientTemperature float64   `json:"ambient_temperature"` // in Kelvin or relative units
	SystemEntropy      float64   `json:"system_entropy"`      // overall thermodynamic chaos
	FreeResourceMotes  int       `json:"free_resource_motes"` // ambient energy available in space
	ActivePolicy       string    `json:"active_policy"`       // symbiosis, competition, entropy_balance
	TickCount          uint64    `json:"tick_count"`
	LastEvent          string    `json:"last_event"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// TriggerEventRequest represents an intervention by the researcher.
type TriggerEventRequest struct {
	EventType string  `json:"event_type"` // solar_flare, cryo_wave, em_pulse, resource_cluster
	Intensity float64 `json:"intensity"`  // 0.1 to 2.0 (default 1.0)
}

// UpdateColonyRequest allows fine-tuning colony parameters.
type UpdateColonyRequest struct {
	Energy      *float64 `json:"energy,omitempty"`
	Entropy     *float64 `json:"entropy,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}

package choice

// AllocationRequest represents the slider movement ("Keep for self" vs "Give to neighbor").
type AllocationRequest struct {
	SourceColonyID    string  `json:"source_colony_id"`
	SourceColonyCamel string  `json:"sourceColonyId"`
	TargetColonyID    string  `json:"target_colony_id"`
	TargetColonyCamel string  `json:"targetColonyId"`
	SharePercent      float64 `json:"share_percent"`
	SharePercentCamel float64 `json:"sharePercent"`
	Policy            string  `json:"policy"`
}

// AllocationResponse is the real-time physical outcome calculated by the Choice Machine.
type AllocationResponse struct {
	SourceColonyID    string  `json:"source_colony_id"`
	TargetColonyID    string  `json:"target_colony_id"`
	TransferredEnergy float64 `json:"transferred_energy"`
	SourceNewEnergy   float64 `json:"source_new_energy"`
	TargetNewEnergy   float64 `json:"target_new_energy"`
	SystemWelfare     float64 `json:"system_welfare"` // 0-100%
	SystemEntropy     float64 `json:"system_entropy"` // 0-100%
	BeamActive        bool    `json:"beam_active"`
	Message           string  `json:"message"`
	RatioTelemetry    string  `json:"ratio_telemetry"`
}

// Dilemma represents a crisis scenario for collective decision making.
type Dilemma struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Alternatives []Alternative `json:"alternatives"`
}

// Alternative is a candidate choice in a dilemma.
type Alternative struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	EnergyDeltas map[string]float64 `json:"energy_deltas"` // colony_id -> delta
	EntropyDelta float64            `json:"entropy_delta"`
}

// ResolveDilemmaRequest submits a dilemma to the Choice Machine with a chosen philosophy.
type ResolveDilemmaRequest struct {
	DilemmaID      string `json:"dilemma_id"`
	DilemmaIDCamel string `json:"dilemmaId"`
	Philosophy     string `json:"philosophy"` // bentham, rawls, quadratic, entropy
}

// ResolveDilemmaResponse returns the calculated consensus.
type ResolveDilemmaResponse struct {
	DilemmaID            string              `json:"dilemma_id"`
	Philosophy           string              `json:"philosophy"`
	WinningAlternativeID string              `json:"winning_alternative_id"`
	WinningTitle         string              `json:"winning_title"`
	SystemWelfare        float64             `json:"system_welfare"`
	SystemEntropy        float64             `json:"system_entropy"`
	ColonyReactions      map[string]Reaction `json:"colony_reactions"`
	Explanation          string              `json:"explanation"`
}

// Reaction represents how a specific colony responded physically to the decision.
type Reaction struct {
	EnergyChange float64 `json:"energy_change"`
	Status       string  `json:"status"`
	ReactionText string  `json:"reaction_text"`
}

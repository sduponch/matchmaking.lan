package gamelog

import "time"

// Event is the unified output of the pattern registry.
// Type follows dot-notation: "cs2.kill", "cs2.round.start", etc.
// Fields contains all named captures from the matched pattern.
// Extra carries complex payloads (e.g. player stats from round_stats JSON).
type Event struct {
	Type   string            `json:"type"`
	Server string            `json:"server"`
	At     time.Time         `json:"at"`
	Fields map[string]string `json:"fields,omitempty"`
	Extra  any               `json:"extra,omitempty"`
}

// RoundStats is the payload for cs2.round.stats events.
type RoundStats struct {
	Round   int                       `json:"round"`
	ScoreCT int                       `json:"score_ct"`
	ScoreT  int                       `json:"score_t"`
	Map     string                    `json:"map"`
	Players map[string]*RoundPlayer   `json:"players"`
}

type RoundPlayer struct {
	AccountID string `json:"accountid"`
	Team      int    `json:"team"`
	Money     int    `json:"money"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
	Damage    int    `json:"dmg"`
	HSP       string `json:"hsp"`
	ADR       string `json:"adr"`
	MVP       int    `json:"mvp"`
}

package match

import "time"

type Phase string

const (
	PhaseIdle     Phase = "idle"
	PhaseWarmup   Phase = "warmup"
	PhaseKnife    Phase = "knife"
	PhaseLive     Phase = "live"
	PhaseHalfTime Phase = "halftime"
	PhaseOvertime Phase = "overtime"
	PhaseGameOver Phase = "game_over"
)

type PlayerStat struct {
	Name    string `json:"name"`
	SteamID string `json:"steamid"`
	Team    string `json:"team"`
	Kills   int    `json:"kills"`
	Deaths  int    `json:"deaths"`
	Assists int    `json:"assists"`
	Damage  int    `json:"dmg"`
	HSP     string `json:"hsp,omitempty"`
	ADR     string `json:"adr,omitempty"`
	Money   int    `json:"money"`
	MVP     int    `json:"mvp"`
}

type MatchState struct {
	Phase     Phase                  `json:"phase"`
	Map       string                 `json:"map,omitempty"`
	Round     int                    `json:"round"`
	ScoreCT   int                    `json:"score_ct"`
	ScoreT    int                    `json:"score_t"`
	Players   map[string]*PlayerStat `json:"players"` // steamid → stat
	StartedAt *time.Time             `json:"started_at,omitempty"`
}

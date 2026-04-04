package encounter

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

const storeFile = "encounters.json"

type Encounter struct {
	ID           string     `json:"id"`
	Team1        string     `json:"team1"`                    // team ID
	Team2        string     `json:"team2"`                    // team ID
	Format       string     `json:"format"`                   // "bo1" | "bo3" | "bo5"
	GameMode     string     `json:"game_mode"`                // "defuse" | "casual" | "wingman" | ...
	SidePick     string     `json:"side_pick"`                // "knife" | "ct" | "t"
	LaunchMode   string     `json:"launch_mode"`              // "manual" | "scheduled" | "ready"
	ReadyCount   int        `json:"ready_count,omitempty"`    // players needed when launch_mode = "ready"
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`   // when launch_mode = "scheduled"
	PickBan      bool       `json:"pick_ban"`                 // map selection via pick & ban
	MapPool      []string   `json:"map_pool,omitempty"`       // eligible maps for pick & ban
	VetoFirst    string     `json:"veto_first,omitempty"`     // who starts the veto: "seed" | "toss" | "chifoumi"
	DeciderSide  string     `json:"decider_side,omitempty"`   // side pick on decider map: "pickban" | "toss" | "knife" | "vote"
	MaxRounds    int        `json:"max_rounds,omitempty"`     // mp_maxrounds (default 24 = 12 per half)
	Prac         bool       `json:"prac,omitempty"`           // mp_ignore_round_win_conditions 1 — both sides always play all rounds
	Overtime     bool       `json:"overtime"`                 // whether overtime is enabled (false = draws allowed)
	OTStartMoney int        `json:"ot_start_money,omitempty"` // starting money per player in OT (default 10000)
	MaxOvertimes int        `json:"max_overtimes,omitempty"`  // max OT periods (0 = unlimited)
	TacticalTimeouts    int `json:"tactical_timeouts"`        // timeouts per team (mp_team_timeout_max, default 4)
	TacticalTimeoutTime int `json:"tactical_timeout_time"`    // timeout duration in seconds (mp_team_timeout_time, default 30)
	TacticalTimeoutsOT  int `json:"tactical_timeouts_ot,omitempty"` // timeouts per team in overtime (mp_team_timeout_ot_max, default 1)
	Status       string     `json:"status"`                   // "scheduled" | "live" | "completed"
	ServerID     string     `json:"server_id,omitempty"`      // server token
	ProfileID    string     `json:"profile_id,omitempty"`     // match profile ID
	Hostname     string     `json:"hostname,omitempty"`       // base hostname without phase suffix
	Maps         []GameMap  `json:"maps"`
	Winner       string     `json:"winner,omitempty"`         // "team1" | "team2"
	DemoStatus   string     `json:"demo_status"`              // "none" | "recording"
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

type GameMap struct {
	Number    int    `json:"number"`
	Map       string `json:"map"`
	Score1    int    `json:"score1"`            // score team1 side (CT at start)
	Score2    int    `json:"score2"`            // score team2 side (T at start)
	Winner    string `json:"winner,omitempty"`  // "team1" | "team2"
	Status    string `json:"status"`            // "pending" | "live" | "completed"
	StartSide string `json:"start_side,omitempty"` // team1's starting side: "ct" | "t"
}

// OnComplete is wired in main.go (to phase.CheckRoundComplete once implemented).
var OnComplete func(enc *Encounter)

// OnStart is called just before the changelevel RCON is sent. serverToken identifies the server.
// Wired in main.go to match.ExpectWarmup so the warmup CFG is pushed on the next cs2.map.started.
var OnStart func(serverToken string)

var (
	mu    sync.RWMutex
	store map[string]*Encounter
)

func init() {
	load()
}

func load() {
	store = map[string]*Encounter{}
	data, err := os.ReadFile(storeFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &store)
}

func save() error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storeFile, data, 0644)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// List returns all encounters sorted by creation date (newest first).
func List() []*Encounter {
	mu.RLock()
	defer mu.RUnlock()
	list := make([]*Encounter, 0, len(store))
	for _, e := range store {
		list = append(list, e)
	}
	// Sort newest first
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].CreatedAt.After(list[i].CreatedAt) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

// Get returns an encounter by ID.
func Get(id string) (*Encounter, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := store[id]
	return e, ok
}

// GetByServerID returns the live encounter for a given server token, if any.
func GetByServerID(serverID string) (*Encounter, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, e := range store {
		if e.ServerID == serverID && e.Status == "live" {
			return e, true
		}
	}
	return nil, false
}

// Create creates a new encounter with generated maps placeholders.
func Create(enc *Encounter) error {
	mu.Lock()
	defer mu.Unlock()
	enc.ID = newID()
	enc.Status = "scheduled"
	enc.DemoStatus = "none"
	if enc.GameMode == "" {
		enc.GameMode = "defuse"
	}
	enc.CreatedAt = time.Now()
	enc.Maps = buildMaps(enc.Format, enc.Maps)
	store[enc.ID] = enc
	return save()
}

// Update replaces encounter metadata (preserves status, maps, winner, dates).
func Update(id string, patch *Encounter) error {
	mu.Lock()
	defer mu.Unlock()
	existing, ok := store[id]
	if !ok {
		return os.ErrNotExist
	}
	patch.ID = id
	patch.Status = existing.Status
	patch.Maps = existing.Maps
	patch.Winner = existing.Winner
	patch.DemoStatus = existing.DemoStatus
	patch.CreatedAt = existing.CreatedAt
	patch.StartedAt = existing.StartedAt
	patch.EndedAt = existing.EndedAt
	store[id] = patch
	return save()
}

// Delete removes an encounter.
func Delete(id string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := store[id]; !ok {
		return os.ErrNotExist
	}
	delete(store, id)
	return save()
}

// Start assigns server + profile and marks the encounter live.
func Start(id, serverID, profileID, hostname string) error {
	mu.Lock()
	defer mu.Unlock()
	enc, ok := store[id]
	if !ok {
		return os.ErrNotExist
	}
	now := time.Now()
	enc.ServerID = serverID
	enc.ProfileID = profileID
	enc.Hostname = hostname
	enc.Status = "live"
	enc.DemoStatus = "recording"
	enc.StartedAt = &now
	// Mark first map as live
	for i := range enc.Maps {
		if enc.Maps[i].Status == "pending" {
			enc.Maps[i].Status = "live"
			break
		}
	}
	if err := save(); err != nil {
		return err
	}
	// Arm the warmup CFG push before changelevel is sent.
	if OnStart != nil {
		OnStart(serverID)
	}
	return nil
}

// PhaseHostname returns the full hostname for a given phase.
func (e *Encounter) PhaseHostname(phase string) string {
	labels := map[string]string{
		"warmup":    "Warmup",
		"knife":     "Couteaux",
		"live":      "Live",
		"halftime":  "Mi-temps",
		"overtime":  "Prolongation",
		"game_over": "Terminé",
	}
	label, ok := labels[phase]
	if !ok {
		return e.Hostname
	}
	return e.Hostname + " - " + label
}

// RecordResult is called by the match.OnGameOver hook via main.go.
// serverToken is the token of the server; scoreCT and scoreT are the final halftime scores.
func RecordResult(serverToken string, scoreCT, scoreT int) {
	mu.Lock()

	var enc *Encounter
	for _, e := range store {
		if e.ServerID == serverToken && e.Status == "live" {
			enc = e
			break
		}
	}
	if enc == nil {
		mu.Unlock()
		return
	}

	// Find the current live map
	mapIdx := -1
	for i := range enc.Maps {
		if enc.Maps[i].Status == "live" {
			mapIdx = i
			break
		}
	}
	if mapIdx < 0 {
		mu.Unlock()
		return
	}

	m := &enc.Maps[mapIdx]
	m.Score1 = scoreCT
	m.Score2 = scoreT
	m.Status = "completed"
	if scoreCT > scoreT {
		m.Winner = "team1"
	} else if scoreT > scoreCT {
		m.Winner = "team2"
	}

	// Count map wins
	wins1, wins2 := countWins(enc.Maps)
	needed := winsNeeded(enc.Format)

	var completed bool
	if wins1 >= needed {
		enc.Winner = "team1"
		completed = true
	} else if wins2 >= needed {
		enc.Winner = "team2"
		completed = true
	}

	if completed {
		now := time.Now()
		enc.Status = "completed"
		enc.EndedAt = &now
		enc.DemoStatus = "none"
	} else {
		// Activate next map
		for i := range enc.Maps {
			if enc.Maps[i].Status == "pending" {
				enc.Maps[i].Status = "live"
				break
			}
		}
	}

	_ = save()

	var toNotify *Encounter
	if completed && OnComplete != nil {
		cp := *enc
		toNotify = &cp
	}
	mu.Unlock()

	if toNotify != nil {
		OnComplete(toNotify)
	}
}

// SetResult is an admin override for a specific map result.
func SetResult(encID string, mapNumber int, score1, score2 int) error {
	mu.Lock()
	defer mu.Unlock()
	enc, ok := store[encID]
	if !ok {
		return os.ErrNotExist
	}
	for i := range enc.Maps {
		if enc.Maps[i].Number == mapNumber {
			enc.Maps[i].Score1 = score1
			enc.Maps[i].Score2 = score2
			enc.Maps[i].Status = "completed"
			if score1 > score2 {
				enc.Maps[i].Winner = "team1"
			} else if score2 > score1 {
				enc.Maps[i].Winner = "team2"
			}
			break
		}
	}
	wins1, wins2 := countWins(enc.Maps)
	needed := winsNeeded(enc.Format)
	if wins1 >= needed {
		enc.Winner = "team1"
		now := time.Now()
		enc.Status = "completed"
		enc.EndedAt = &now
	} else if wins2 >= needed {
		enc.Winner = "team2"
		now := time.Now()
		enc.Status = "completed"
		enc.EndedAt = &now
	}
	return save()
}

func buildMaps(format string, provided []GameMap) []GameMap {
	total := formatTotal(format)
	maps := make([]GameMap, total)
	for i := 0; i < total; i++ {
		maps[i] = GameMap{Number: i + 1, Status: "pending"}
		if i < len(provided) {
			maps[i].Map = provided[i].Map
		}
	}
	if total > 0 {
		maps[0].Status = "pending" // will be set to live on Start
	}
	return maps
}

func formatTotal(format string) int {
	switch format {
	case "bo3":
		return 3
	case "bo5":
		return 5
	default:
		return 1
	}
}

func winsNeeded(format string) int {
	return (formatTotal(format) / 2) + 1
}

func countWins(maps []GameMap) (int, int) {
	w1, w2 := 0, 0
	for _, m := range maps {
		switch m.Winner {
		case "team1":
			w1++
		case "team2":
			w2++
		}
	}
	return w1, w2
}

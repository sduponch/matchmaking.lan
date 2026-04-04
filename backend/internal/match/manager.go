package match

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/gamelog"
)

// GetLastLogAt is set at startup to retrieve last log reception time per server.
var GetLastLogAt func(addr string) *time.Time

// OnGameOver is called when a match ends. Wired in main.go to encounter.RecordResult.
var OnGameOver func(serverAddr string, scoreCT, scoreT int)

// OnPhaseChange is called on phase transitions. Wired in main.go to push CFG + hostname via RCON.
var OnPhaseChange func(serverAddr, phase string)

// GetEncounterInfo returns the encounter settings for the server's current live encounter.
// sidePick is "knife" | "ct" | "t". readyCount is the required !ready count (0 = all connected players).
// maxRounds is mp_maxrounds (0 → default 24). Wired in main.go.
var GetEncounterInfo func(serverAddr string) (sidePick string, readyCount int, maxRounds int, ok bool)

// OnKnifeChoice is called when the knife winner picks a side.
// winnerSide is "CT" or "TERRORIST". chosenSide is "ct" or "t".
// Wired in main.go to optionally send mp_swapteams then push live.cfg.
var OnKnifeChoice func(serverAddr, winnerSide, chosenSide string)

// OnPlayerJoinTeam is called when a human player manually joins a team during warmup.
// uid is the server-local player UID (for kickid). team is "CT" or "TERRORIST".
// Fired for every player switching from Unassigned during PhaseWarmup.
var OnPlayerJoinTeam func(serverAddr, uid, steamID, playerName, team string)

// ExpectWarmup arms the warmup CFG push for the next cs2.map.started event on serverAddr.
// Call this just before sending the changelevel RCON for a new encounter.
func ExpectWarmup(serverAddr string) {
	m := Get(serverAddr)
	m.mu.Lock()
	m.expectWarmup = true
	m.mu.Unlock()
}

var machines sync.Map // serverAddr → *Machine

func Get(serverAddr string) *Machine {
	v, _ := machines.LoadOrStore(serverAddr, newMachine())
	return v.(*Machine)
}

// Apply dispatches a gamelog event to the relevant server's state machine.
func Apply(e *gamelog.Event) {
	Get(e.Server).Apply(e)
}

// HandleGetState returns the current match state for a server.
func HandleGetState() gin.HandlerFunc {
	return func(c *gin.Context) {
		addr := c.GetString("serverAddr")
		state := Get(addr).State()
		if GetLastLogAt != nil {
			c.JSON(http.StatusOK, gin.H{
				"phase":       state.Phase,
				"map":         state.Map,
				"round":       state.Round,
				"score_ct":    state.ScoreCT,
				"score_t":     state.ScoreT,
				"players":     state.Players,
				"started_at":  state.StartedAt,
				"last_log_at": GetLastLogAt(addr),
			})
		} else {
			c.JSON(http.StatusOK, state)
		}
	}
}

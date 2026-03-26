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
		addr := c.Param("addr")
		state := Get(addr).State()
		if GetLastLogAt != nil {
			c.JSON(http.StatusOK, gin.H{
				"phase":      state.Phase,
				"map":        state.Map,
				"round":      state.Round,
				"score_ct":   state.ScoreCT,
				"score_t":    state.ScoreT,
				"players":    state.Players,
				"started_at": state.StartedAt,
				"last_log_at": GetLastLogAt(addr),
			})
		} else {
			c.JSON(http.StatusOK, state)
		}
	}
}

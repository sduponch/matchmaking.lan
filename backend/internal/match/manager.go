package match

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/gamelog"
)

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
		c.JSON(http.StatusOK, state)
	}
}

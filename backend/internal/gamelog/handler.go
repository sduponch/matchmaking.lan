package gamelog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleSSE streams game events for a server via Server-Sent Events.
func HandleSSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		serverAddr := c.GetString("serverAddr")

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		sub := Broker.Subscribe(serverAddr)
		defer Broker.Unsubscribe(serverAddr, sub)

		c.Stream(func(w io.Writer) bool {
			select {
			case e, ok := <-sub:
				if !ok {
					return false
				}
				data, _ := json.Marshal(e)
				fmt.Fprintf(w, "data: %s\n\n", data)
				c.Writer.(http.Flusher).Flush()
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}

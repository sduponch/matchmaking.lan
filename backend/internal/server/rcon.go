package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorcon/rcon"
)

func sendRCON(addr, password, command string) (string, error) {
	conn, err := rcon.Dial(addr, password)
	if err != nil {
		return "", fmt.Errorf("rcon connection failed: %w", err)
	}
	defer conn.Close()

	resp, err := conn.Execute(command)
	if err != nil {
		return "", fmt.Errorf("rcon command failed: %w", err)
	}
	return resp, nil
}

func HandleChangeMap() gin.HandlerFunc {
	return func(c *gin.Context) {
		addr := c.Param("addr")

		var body struct {
			Map string `json:"map" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		managedMap := getManagedAddrs()
		password, ok := managedMap[addr]
		if !ok || password == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "server not managed or no RCON password"})
			return
		}

		resp, err := sendRCON(addr, password, "changelevel "+body.Map)
		if err != nil {
			log.Printf("[rcon] %s changelevel %s: %v", addr, body.Map, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[rcon] %s changelevel %s → %q", addr, body.Map, resp)
		c.JSON(http.StatusOK, gin.H{"map": body.Map})
	}
}

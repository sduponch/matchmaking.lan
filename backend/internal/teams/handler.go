package teams

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/registry"
)

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, List())
	}
}

func HandleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}
		t, err := Create(body.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, t)
	}
}

func HandleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		// Remove team from all players in registry
		if t, ok := Get(id); ok {
			for _, steamid := range t.Players {
				registry.SetTeam(steamid, "")
			}
		}
		if err := Delete(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func HandleAddPlayer() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			SteamID string `json:"steamid" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "steamid required"})
			return
		}
		if err := AddPlayer(id, body.SteamID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		// Update player's team in registry
		if t, ok := Get(id); ok {
			registry.SetTeam(body.SteamID, t.Name)
		}
		c.Status(http.StatusNoContent)
	}
}

func HandleRemovePlayer() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		steamid := c.Param("steamid")
		if err := RemovePlayer(id, steamid); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		registry.SetTeam(steamid, "")
		c.Status(http.StatusNoContent)
	}
}

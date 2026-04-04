package encounter

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/matchconfig"
	"matchmaking.lan/backend/internal/server"
	"matchmaking.lan/backend/internal/teams"
)

// GameModeCommands returns the game_type/game_mode RCON commands for an encounter game mode.
func GameModeCommands(mode string) []string {
	switch mode {
	case "casual":
		return []string{"game_type 0", "game_mode 0"}
	case "wingman":
		return []string{"game_type 0", "game_mode 2"}
	case "retakes":
		return []string{"game_type 0", "game_mode 1", "mp_retakes 1"}
	case "armsrace":
		return []string{"game_type 1", "game_mode 0"}
	case "deathmatch":
		return []string{"game_type 1", "game_mode 2"}
	default: // "defuse", "hostage" or empty — competitive mode, map prefix determines rules
		return []string{"game_type 0", "game_mode 1", "mp_retakes 0"}
	}
}

// EncounterResponse enriches an Encounter with team names.
type EncounterResponse struct {
	*Encounter
	Team1Name string `json:"team1_name,omitempty"`
	Team2Name string `json:"team2_name,omitempty"`
}

func enrich(enc *Encounter) EncounterResponse {
	r := EncounterResponse{Encounter: enc}
	if t, ok := teams.Get(enc.Team1); ok {
		r.Team1Name = t.Name
	}
	if t, ok := teams.Get(enc.Team2); ok {
		r.Team2Name = t.Name
	}
	return r
}

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		list := List()
		resp := make([]EncounterResponse, len(list))
		for i, e := range list {
			resp[i] = enrich(e)
		}
		c.JSON(http.StatusOK, resp)
	}
}

func HandleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var enc Encounter
		if err := c.ShouldBindJSON(&enc); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if enc.Team1 == "" || enc.Team2 == "" || enc.Format == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "team1, team2 and format are required"})
			return
		}
		if err := Create(&enc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, enrich(&enc))
	}
}

func HandleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		enc, ok := Get(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		c.JSON(http.StatusOK, enrich(enc))
	}
}

func HandleUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var patch Encounter
		if err := c.ShouldBindJSON(&patch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := Update(id, &patch); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		enc, _ := Get(id)
		c.JSON(http.StatusOK, enrich(enc))
	}
}

func HandleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := Delete(c.Param("id")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// HandleStart assigns server + profile, pushes warmup CFG, sets hostname, starts demo recording.
func HandleStart() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			ServerID  string `json:"server_id"  binding:"required"`
			ProfileID string `json:"profile_id" binding:"required"`
			Label     string `json:"label"`  // optional prefix, e.g. tournament/event name
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		enc, ok := Get(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		if enc.Status != "scheduled" {
			c.JSON(http.StatusConflict, gin.H{"error": "encounter already started or completed"})
			return
		}

		addr, rcon, ok := server.GetAddrRCON(body.ServerID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "server not found or no RCON configured"})
			return
		}

		// Build base hostname: "[Label - ]Team1 vs Team2"
		name1, name2 := enc.Team1, enc.Team2
		if t, ok := teams.Get(enc.Team1); ok {
			name1 = t.Name
		}
		if t, ok := teams.Get(enc.Team2); ok {
			name2 = t.Name
		}
		baseHostname := name1 + " vs " + name2
		if body.Label != "" {
			baseHostname = body.Label + " - " + baseHostname
		}

		// Switch to the right game mode, then warmup CFG, hostname, changelevel (must be last).
		cmds := GameModeCommands(enc.GameMode)
		cmds = append(cmds, matchconfig.GetProfileWarmupCommands(body.ProfileID)...)
		cmds = append(cmds, `hostname "`+baseHostname+` - Warmup"`)

		// Re-register the log endpoint before changelevel so CS2 sends logs after map reload.
		cmds = append(cmds, server.GetLogSetupCmds(body.ServerID)...)

		// Changelevel to the first map if defined — must be the last command.
		if len(enc.Maps) > 0 && enc.Maps[0].Map != "" {
			cmds = append(cmds, "changelevel "+enc.Maps[0].Map)
		}

		if err := server.SendRCONBatch(addr, rcon, cmds); err != nil {
			log.Printf("[encounter] %s start RCON ERROR: %v", id, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "RCON failed: " + err.Error()})
			return
		}

		if err := Start(id, body.ServerID, body.ProfileID, baseHostname); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		enc, _ = Get(id)
		log.Printf("[encounter] %s started on server %s (profile %s)", id, body.ServerID, body.ProfileID)
		c.JSON(http.StatusOK, enrich(enc))
	}
}

// HandleSetResult is an admin override for a specific map result.
func HandleSetResult() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			MapNumber int `json:"map_number" binding:"required"`
			Score1    int `json:"score1"`
			Score2    int `json:"score2"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := SetResult(id, body.MapNumber, body.Score1, body.Score2); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		enc, _ := Get(id)
		c.JSON(http.StatusOK, enrich(enc))
	}
}

// HandleReopen resets a completed or live encounter back to scheduled.
func HandleReopen() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		enc, ok := Get(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		mu.Lock()
		enc.Status = "scheduled"
		enc.Winner = ""
		enc.ServerID = ""
		enc.ProfileID = ""
		enc.StartedAt = nil
		enc.EndedAt = nil
		enc.DemoStatus = "none"
		for i := range enc.Maps {
			enc.Maps[i].Score1 = 0
			enc.Maps[i].Score2 = 0
			enc.Maps[i].Winner = ""
			if i == 0 {
				enc.Maps[i].Status = "pending"
			} else {
				enc.Maps[i].Status = "pending"
			}
		}
		_ = save()
		mu.Unlock()
		c.JSON(http.StatusOK, enrich(enc))
	}
}

// HandleSetMaps replaces the map list for a scheduled encounter (veto result).
func HandleSetMaps() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		enc, ok := Get(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		if enc.Status != "scheduled" {
			c.JSON(http.StatusConflict, gin.H{"error": "encounter already started"})
			return
		}
		var maps []GameMap
		if err := c.ShouldBindJSON(&maps); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		mu.Lock()
		enc.Maps = maps
		_ = save()
		mu.Unlock()
		c.JSON(http.StatusOK, enrich(enc))
	}
}

// HandleListMaps returns the map list with win counts.
func HandleListMaps() gin.HandlerFunc {
	return func(c *gin.Context) {
		enc, ok := Get(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
			return
		}
		w1, w2 := countWins(enc.Maps)
		c.JSON(http.StatusOK, gin.H{
			"maps":   enc.Maps,
			"wins1":  w1,
			"wins2":  w2,
			"needed": strconv.Itoa(winsNeeded(enc.Format)),
		})
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/auth"
	"matchmaking.lan/backend/internal/bot"
	"matchmaking.lan/backend/internal/config"
	"matchmaking.lan/backend/internal/encounter"
	"matchmaking.lan/backend/internal/gamelog"
	"matchmaking.lan/backend/internal/match"
	"matchmaking.lan/backend/internal/mappoolconfig"
	"matchmaking.lan/backend/internal/matchconfig"
	"matchmaking.lan/backend/internal/player"
	"matchmaking.lan/backend/internal/registry"
	"matchmaking.lan/backend/internal/server"
	"matchmaking.lan/backend/internal/teams"
)

func main() {
	config.Load()
	registry.SyncRoles(config.C.AdminSteamIDs)

	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		log.Printf("[GIN-debug] %-6s %-30s --> %s (%d handlers)", httpMethod, absolutePath, handlerName, nuHandlers)
	}

	ctx := context.Background()
	botManager := bot.NewManager(config.C.BotPort)
	botManager.Start(ctx)
	gamelog.OnEvent = match.Apply
	gamelog.ResolveToken = server.GetAddrByToken
	gamelog.OnLog = server.UpdateLastLog
	match.GetLastLogAt = server.GetLastLogAt
	encounter.OnStart = func(serverToken string) {
		if addr, ok := server.GetAddrByToken(serverToken); ok {
			match.ExpectWarmup(addr)
		}
	}
	match.OnGameOver = func(addr string, scoreCT, scoreT int) {
		if token, ok := server.GetTokenByAddr(addr); ok {
			encounter.RecordResult(token, scoreCT, scoreT)
		}
	}
	match.GetEncounterInfo = func(addr string) (sidePick string, readyCount int, maxRounds int, ok bool) {
		token, tok := server.GetTokenByAddr(addr)
		if !tok {
			return
		}
		enc, eok := encounter.GetByServerID(token)
		if !eok {
			return
		}
		mr := enc.MaxRounds
		if mr == 0 {
			mr = 24
		}
		return enc.SidePick, enc.ReadyCount, mr, true
	}

	// getTeamNames returns the display names for Team1 and Team2.
	getTeamNames := func(enc *encounter.Encounter) (string, string) {
		name1, name2 := "Team1", "Team2"
		if t, ok := teams.Get(enc.Team1); ok && t.Name != "" {
			name1 = t.Name
		}
		if t, ok := teams.Get(enc.Team2); ok && t.Name != "" {
			name2 = t.Name
		}
		return name1, name2
	}

	// teamNameCmds returns mp_teamname_1/2 commands (no side awareness, for warmup/knife).
	// mp_teamname_1 = CT slot, mp_teamname_2 = T slot.
	teamNameCmds := func(enc *encounter.Encounter) []string {
		name1, name2 := getTeamNames(enc)
		return []string{
			`mp_teamname_1 "` + name1 + `"`,
			`mp_teamname_2 "` + name2 + `"`,
		}
	}

	// teamNameCmdsWithSide returns mp_teamname commands with correct CT/T assignment.
	// ctIsTeam1=true → Team1 is on CT side; false → Team2 is on CT side.
	teamNameCmdsWithSide := func(enc *encounter.Encounter, ctIsTeam1 bool) []string {
		name1, name2 := getTeamNames(enc)
		if ctIsTeam1 {
			return []string{`mp_teamname_1 "` + name1 + `"`, `mp_teamname_2 "` + name2 + `"`}
		}
		return []string{`mp_teamname_1 "` + name2 + `"`, `mp_teamname_2 "` + name1 + `"`}
	}

	// buildHostname builds the hostname for any phase.
	// For phases with known sides (live, halftime, second_half, overtime), adds (CT)/(T) labels.
	// For other phases (warmup, knife, game_over), shows "Team1 vs Team2 - Label".
	buildHostname := func(enc *encounter.Encounter, ctIsTeam1 bool, phase string) string {
		name1, name2 := getTeamNames(enc)
		// Modes without halves show "Live" instead of "1ère Mi-temps".
		firstHalfLabel := "1ère Mi-temps"
		switch enc.GameMode {
		case "deathmatch", "armsrace", "casual":
			firstHalfLabel = "Live"
		}
		sideLabels := map[string]string{
			"warmup":      "Warmup",
			"first_half":  firstHalfLabel,
			"second_half": "2ème Mi-temps",
			"halftime":    "Mi-temps",
			"overtime":    "Prolongation",
		}
		phaseLabels := map[string]string{
			"knife":     "Couteaux",
			"game_over": "Terminé",
		}
		if label, ok := sideLabels[phase]; ok {
			var ctName, tName string
			if ctIsTeam1 {
				ctName, tName = name1, name2
			} else {
				ctName, tName = name2, name1
			}
			return ctName + " (CT) vs " + tName + " (T) - " + label
		}
		h := name1 + " vs " + name2
		if label, ok := phaseLabels[phase]; ok {
			h += " - " + label
		}
		return h
	}

	match.OnPlayerJoinTeam = func(addr, uid, steamID, playerName, team string) {
		token, ok := server.GetTokenByAddr(addr)
		if !ok {
			return
		}
		enc, ok := encounter.GetByServerID(token)
		if !ok {
			return
		}
		// Only enforce sides when a fixed side is configured.
		if enc.SidePick != "ct" && enc.SidePick != "t" {
			return
		}
		// Determine the correct side for this Steam ID.
		// enc.SidePick=="ct" → team1 is CT, team2 is T.
		t1, ok1 := teams.Get(enc.Team1)
		t2, ok2 := teams.Get(enc.Team2)
		var correctSide string
		if ok1 {
			for _, id := range t1.Players {
				if id == steamID {
					if enc.SidePick == "ct" {
						correctSide = "CT"
					} else {
						correctSide = "TERRORIST"
					}
					break
				}
			}
		}
		if correctSide == "" && ok2 {
			for _, id := range t2.Players {
				if id == steamID {
					if enc.SidePick == "ct" {
						correctSide = "TERRORIST"
					} else {
						correctSide = "CT"
					}
					break
				}
			}
		}
		if correctSide == "" || team == correctSide {
			return // player not assigned to either team, or already on the right side
		}
		srvAddr, rcon, ok := server.GetAddrRCON(token)
		if !ok {
			return
		}
		// Count players currently in a team (CT or T).
		// If this player is alone, swap all teams instead of kicking.
		state := match.Get(addr).State()
		playersInTeam := 0
		for _, p := range state.Players {
			if p.Team == "CT" || p.Team == "TERRORIST" {
				playersInTeam++
			}
		}
		expectedCT := enc.SidePick == "ct"
		if playersInTeam <= 1 {
			log.Printf("[match] %s wrong team: %s joined %s, alone → swapping", addr, playerName, team)
			_ = server.SendRCONBatch(srvAddr, rcon, []string{"mp_swapteams"})
			nameCmds := teamNameCmdsWithSide(enc, expectedCT)
			go func() {
				time.Sleep(2 * time.Second)
				_ = server.SendRCONBatch(srvAddr, rcon, nameCmds)
			}()
		} else {
			sideLabel := "CT"
			if correctSide == "TERRORIST" {
				sideLabel = "T"
			}
			log.Printf("[match] %s wrong team: %s joined %s, expected %s → kicking", addr, playerName, team, correctSide)
			_ = server.SendRCONBatch(srvAddr, rcon, []string{
				`say "[ Equipe ] ` + playerName + `: rejoins les ` + sideLabel + ` !"`,
				"kickid " + uid,
			})
		}
	}

	match.OnKnifeChoice = func(addr, winnerSide, chosenSide string) {
		token, ok := server.GetTokenByAddr(addr)
		if !ok {
			return
		}
		enc, ok := encounter.GetByServerID(token)
		if !ok {
			return
		}
		srvAddr, rcon, ok := server.GetAddrRCON(token)
		if !ok {
			return
		}
		var cmds []string
		// Swap teams if the knife winner wants to switch from their current side.
		needSwap := (winnerSide == "CT" && chosenSide == "t") ||
			(winnerSide == "TERRORIST" && chosenSide == "ct")
		if needSwap {
			cmds = append(cmds, "mp_swapteams")
		}
		// After potential swap: Team1 is CT if no swap happened (knife started Team1=CT).
		ctIsTeam1 := !needSwap
		cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "live")...)
		cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
		cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "first_half")+`"`)
		_ = server.SendRCONBatch(srvAddr, rcon, cmds)
	}

	match.OnPhaseChange = func(addr, phase string) {
		token, ok := server.GetTokenByAddr(addr)
		if !ok {
			return
		}
		enc, ok := encounter.GetByServerID(token)
		if !ok {
			return
		}
		srvAddr, rcon, ok := server.GetAddrRCON(token)
		if !ok {
			return
		}
		var cmds []string
		switch phase {
		case "warmup":
			ctIsTeam1 := enc.SidePick != "t"
			cmds = append(cmds, matchconfig.GetProfileWarmupCommands(enc.ProfileID)...)
			cmds = append(cmds, server.GetLogSetupCmds(token)...)
			cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "warmup")+`"`)
		case "warmup_end":
			cmds = []string{`tv_record "enc_` + enc.ID + `"`}
		case "knife":
			cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "knife")...)
			cmds = append(cmds, teamNameCmds(enc)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, true, "knife")+`"`)
		case "first_half":
			// "first_half" via OnPhaseChange = non-knife path (SidePick "ct" or "t").
			ctIsTeam1 := enc.SidePick != "t"
			cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "live")...)
			cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "first_half")+`"`)
		case "second_half":
			// Second half: sides swap vs first half.
			ctIsTeam1 := enc.SidePick == "t"
			cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "second_half")+`"`)
		case "halftime":
			// Same sides as first half.
			ctIsTeam1 := enc.SidePick != "t"
			cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "halftime")...)
			cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "halftime")+`"`)
		case "game_over":
			cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "game_over")...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, true, "game_over")+`"`)
			cmds = append(cmds, "tv_stoprecord")
		case "overtime":
			// Overtime: use first-half side assignment.
			ctIsTeam1 := enc.SidePick != "t"
			cmds = append(cmds, matchconfig.GetProfilePhaseCommands(enc.ProfileID, "overtime")...)
			cmds = append(cmds, teamNameCmdsWithSide(enc, ctIsTeam1)...)
			cmds = append(cmds, `hostname "`+buildHostname(enc, ctIsTeam1, "overtime")+`"`)
		}
		_ = server.SendRCONBatch(srvAddr, rcon, cmds)
	}


	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s [GIN] %d | %s | %s | %s %s\n",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			param.StatusCode,
			param.Latency.Round(time.Millisecond),
			param.ClientIP,
			param.Method,
			param.Path,
		)
	}))
	r.Use(corsMiddleware())

	r.GET("/auth/steam", auth.HandleCallback)

	r.GET("/servers", requireAuth(), server.HandleList())
	r.POST("/servers", requireAuth(), requireAdmin(), server.HandleAdd())
	r.POST("/internal/log/:token", func(c *gin.Context) { gamelog.HTTPHandler(c.Writer, c.Request) })
	r.POST("/internal/log", func(c *gin.Context) { gamelog.HTTPHandler(c.Writer, c.Request) })

	srv := r.Group("/servers/:token", resolveServerToken())
	srv.POST("/map", requireAuth(), requireAdmin(), server.HandleChangeMap())
	srv.POST("/cfg", requireAuth(), requireAdmin(), server.HandlePushCFG())
	srv.GET("/match", requireAuth(), match.HandleGetState())
	srv.GET("/logs", requireAuth(), gamelog.HandleSSE())
	srv.DELETE("", requireAuth(), requireAdmin(), server.HandleRemove())
	srv.PUT("/name", requireAuth(), requireAdmin(), server.HandleSetName())

	r.GET("/players", requireAuth(), requireAdmin(), registry.HandleList())

	r.GET("/teams", requireAuth(), teams.HandleList())
	r.POST("/teams", requireAuth(), requireAdmin(), teams.HandleCreate())
	r.DELETE("/teams/:id", requireAuth(), requireAdmin(), teams.HandleDelete())
	r.POST("/teams/:id/players", requireAuth(), requireAdmin(), teams.HandleAddPlayer())
	r.DELETE("/teams/:id/players/:steamid", requireAuth(), requireAdmin(), teams.HandleRemovePlayer())

	r.GET("/match-profiles", requireAuth(), matchconfig.HandleList())
	r.POST("/match-profiles", requireAuth(), requireAdmin(), matchconfig.HandleCreate())
	r.GET("/match-profiles/:id", requireAuth(), matchconfig.HandleGet())
	r.PUT("/match-profiles/:id", requireAuth(), requireAdmin(), matchconfig.HandleUpdate())
	r.DELETE("/match-profiles/:id", requireAuth(), requireAdmin(), matchconfig.HandleDelete())
	r.GET("/match-profiles/:id/cfg/:phase", requireAuth(), matchconfig.HandleGetCFG())
	r.PUT("/match-profiles/:id/cfg/:phase", requireAuth(), requireAdmin(), matchconfig.HandleSetCFG())
	r.GET("/server-init-cfg", requireAuth(), matchconfig.HandleGetServerInitCFG())
	r.PUT("/server-init-cfg", requireAuth(), requireAdmin(), matchconfig.HandleSetServerInitCFG())

	r.GET("/map-pool", requireAuth(), mappoolconfig.HandleGet())
	r.PUT("/map-pool", requireAuth(), requireAdmin(), mappoolconfig.HandleSet())

	r.GET("/encounters", requireAuth(), encounter.HandleList())
	r.POST("/encounters", requireAuth(), requireAdmin(), encounter.HandleCreate())
	r.GET("/encounters/:id", requireAuth(), encounter.HandleGet())
	r.PUT("/encounters/:id", requireAuth(), requireAdmin(), encounter.HandleUpdate())
	r.DELETE("/encounters/:id", requireAuth(), requireAdmin(), encounter.HandleDelete())
	r.POST("/encounters/:id/start", requireAuth(), requireAdmin(), encounter.HandleStart())
	r.PUT("/encounters/:id/maps", requireAuth(), requireAdmin(), encounter.HandleSetMaps())
	r.POST("/encounters/:id/result", requireAuth(), requireAdmin(), encounter.HandleSetResult())
	r.POST("/encounters/:id/reopen", requireAuth(), requireAdmin(), encounter.HandleReopen())
	r.GET("/encounters/:id/maps", requireAuth(), encounter.HandleListMaps())

	r.GET("/profile/:steamid", requireAuth(), player.HandleGetProfile(botManager))
	r.GET("/profile/:steamid/cs2", requireAuth(), player.HandleGetCS2(botManager))
	r.GET("/profile/:steamid/faceit", requireAuth(), player.HandleGetFaceit())

	r.GET("/auth/me", requireAuth(), func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		c.JSON(http.StatusOK, gin.H{
			"steamid":  claims.SteamID,
			"username": claims.Username,
			"avatar":   claims.AvatarURL,
			"role":     claims.Role,
		})
	})

	log.Printf("Backend listening on :%s", config.C.Port)
	r.Run(":" + config.C.Port)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", config.C.FrontendURL)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		if claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Next()
	}
}

func resolveServerToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		addr, ok := server.GetAddrByToken(token)
		if !ok {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		c.Set("serverAddr", addr)
		c.Next()
	}
}

func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if len(header) < 8 || header[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := auth.ParseJWT(header[7:])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

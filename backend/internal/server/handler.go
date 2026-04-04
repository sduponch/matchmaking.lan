package server

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rumblefrog/go-a2s"
	"matchmaking.lan/backend/internal/config"
	"matchmaking.lan/backend/internal/gamelog"
	"matchmaking.lan/backend/internal/matchconfig"
)

type ServerInfo struct {
	Token      string     `json:"id"`
	Addr       string     `json:"addr"`
	Name       string     `json:"name"`
	Map        string     `json:"map"`
	Players    int        `json:"players"`
	Bots       int        `json:"bots"`
	MaxPlayers int        `json:"max_players"`
	PingMs     int        `json:"ping_ms"`
	Online     bool       `json:"online"`
	Managed    bool       `json:"managed"`
	Maps       []string   `json:"maps,omitempty"`
	LastLogAt  *time.Time `json:"last_log_at,omitempty"`
}

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		all := GetAll()

		// Build addr→token map for managed servers
		addrToToken := make(map[string]string, len(all))
		for tok, e := range all {
			addrToToken[e.Addr] = tok
		}

		// Collect all addresses: discovered + managed
		discovered := discoverLAN(1 * time.Second)
		seen := map[string]bool{}
		var allAddrs []string
		for _, addr := range discovered {
			if !seen[addr] {
				seen[addr] = true
				allAddrs = append(allAddrs, addr)
			}
		}
		for _, e := range all {
			if !seen[e.Addr] {
				seen[e.Addr] = true
				allAddrs = append(allAddrs, e.Addr)
			}
		}

		if len(allAddrs) == 0 {
			c.JSON(http.StatusOK, []ServerInfo{})
			return
		}

		results := make([]ServerInfo, len(allAddrs))
		var wg sync.WaitGroup
		for i, addr := range allAddrs {
			wg.Add(1)
			go func(i int, addr string) {
				defer wg.Done()
				info := query(addr)
				if tok, ok := addrToToken[addr]; ok {
					info.Token = tok
					info.Managed = true
					info.LastLogAt = GetLastLogAt(addr)
					if e, exists := GetByToken(tok); exists {
						if e.Name != "" {
							info.Name = e.Name
						}
						info.Maps = e.Maps
					}
				}
				results[i] = info
			}(i, addr)
		}
		wg.Wait()
		c.JSON(http.StatusOK, results)
	}
}

func HandleAdd() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Addr string `json:"addr" binding:"required"`
			RCON string `json:"rcon"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.RCON != "" {
			if _, err := sendRCON(body.Addr, body.RCON, "status"); err != nil {
				log.Printf("[rcon] test failed for %s: %v", body.Addr, err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "RCON inaccessible : " + err.Error()})
				return
			}
		}

		token, err := upsertManaged(body.Addr, body.RCON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Fetch available maps in background (best-effort).
		if body.RCON != "" {
			go func() {
				maps := FetchMaps(body.Addr, body.RCON)
				if len(maps) > 0 {
					setMaps(token, maps)
					log.Printf("[rcon] %s fetched %d maps", body.Addr, len(maps))
				}
			}()
		}

		// Push server init config (gotv + base cvars).
		if cmds := matchconfig.GetServerInitCommands(); len(cmds) > 0 {
			if err := SendRCONBatch(body.Addr, body.RCON, cmds); err != nil {
				log.Printf("[rcon] %s server_init push ERROR: %v", body.Addr, err)
			} else {
				log.Printf("[rcon] %s server_init pushed (%d commands)", body.Addr, len(cmds))
			}
		}

		// Register remote log listener and verify reception.
		if config.C.BackendURL != "" && body.RCON != "" {
			logURL := config.C.BackendURL + "/internal/log/" + token
			cmd := `logaddress_add_http "` + logURL + `"`
			done := make(chan bool, 1)
			go func() { done <- gamelog.ExpectLog(body.Addr, 5*time.Second) }()
			sendRCON(body.Addr, body.RCON, "log on")
			// Clear all previous logaddress registrations before adding the new one.
			// CS2 accumulates logaddress_add_http entries without deduplicating — old
			// entries from prior server-add operations would cause duplicate event processing.
			sendRCON(body.Addr, body.RCON, "logaddress_delall_http")
			if _, err := sendRCON(body.Addr, body.RCON, cmd); err != nil {
				log.Printf("[gamelog] %s logaddress_add_http ERROR: %v", body.Addr, err)
			} else if received := <-done; received {
				log.Printf("[gamelog] %s log reception verified", body.Addr)
			} else {
				log.Printf("[gamelog] %s log reception timeout — check BACKEND_URL and firewall", body.Addr)
			}
		}

		info := query(body.Addr)
		info.Token = token
		info.Managed = true
		info.LastLogAt = GetLastLogAt(body.Addr)
		c.JSON(http.StatusOK, info)
	}
}

func HandleRemove() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if err := removeManaged(token); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func HandleSetName() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		var body struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := setName(token, body.Name); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		// Update hostname on the CS2 server via RCON (best-effort).
		if e, ok := GetByToken(token); ok && e.RCON != "" {
			if _, err := sendRCON(e.Addr, e.RCON, `hostname "`+body.Name+`"`); err != nil {
				log.Printf("[rcon] %s hostname update failed: %v", e.Addr, err)
			}
		}
		c.Status(http.StatusNoContent)
	}
}

// HandlePushCFG pushes server_init.cfg or a match profile's warmup CFG to the server via RCON.
// Body: { "profile_id": "server_init" | "<profile_id>" }
func HandlePushCFG() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		e, ok := GetByToken(token)
		if !ok || e.RCON == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "server not managed or no RCON password"})
			return
		}
		var body struct {
			ProfileID string `json:"profile_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var cmds []string
		if body.ProfileID == "server_init" {
			cmds = matchconfig.GetServerInitCommands()
		} else {
			cmds = matchconfig.GetProfileWarmupCommands(body.ProfileID)
		}

		if len(cmds) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no commands found for this profile"})
			return
		}

		if err := SendRCONBatch(e.Addr, e.RCON, cmds); err != nil {
			log.Printf("[rcon] %s push cfg %q ERROR: %v", e.Addr, body.ProfileID, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[rcon] %s push cfg %q OK (%d commands)", e.Addr, body.ProfileID, len(cmds))
		c.Status(http.StatusNoContent)
	}
}

func query(addr string) ServerInfo {
	client, err := a2s.NewClient(addr, a2s.TimeoutOption(2*time.Second))
	if err != nil {
		log.Printf("[a2s] %-22s offline (%v)", addr, err)
		return ServerInfo{Addr: addr, Online: false}
	}
	defer client.Close()

	start := time.Now()
	info, err := client.QueryInfo()
	ping := int(time.Since(start).Milliseconds())

	if err != nil {
		log.Printf("[a2s] %-22s offline (%v)", addr, err)
		return ServerInfo{Addr: addr, Online: false}
	}

	humans := int(info.Players) - int(info.Bots)
	if humans < 0 {
		humans = 0
	}

	log.Printf("[a2s] %-22s online | %-24s | map=%-20s | players=%d/%d (bots=%d) | ping=%dms",
		addr, info.Name, info.Map, humans, info.MaxPlayers, info.Bots, ping)

	return ServerInfo{
		Addr:       addr,
		Name:       info.Name,
		Map:        info.Map,
		Players:    humans,
		Bots:       int(info.Bots),
		MaxPlayers: int(info.MaxPlayers),
		PingMs:     ping,
		Online:     true,
	}
}

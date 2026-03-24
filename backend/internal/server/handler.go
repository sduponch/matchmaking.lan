package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rumblefrog/go-a2s"
	"matchmaking.lan/backend/internal/config"
)

type ServerInfo struct {
	Addr       string `json:"addr"`
	Name       string `json:"name"`
	Map        string `json:"map"`
	Players    int    `json:"players"`
	Bots       int    `json:"bots"`
	MaxPlayers int    `json:"max_players"`
	PingMs     int    `json:"ping_ms"`
	Online     bool   `json:"online"`
	Managed    bool   `json:"managed"`
}

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		managedMap := getManagedAddrs()

		// Broadcast discovery + managed servers
		discovered := discoverLAN(1 * time.Second)

		seen := map[string]bool{}
		var allAddrs []string

		for _, addr := range discovered {
			if !seen[addr] {
				seen[addr] = true
				allAddrs = append(allAddrs, addr)
			}
		}
		for addr := range managedMap {
			if !seen[addr] {
				seen[addr] = true
				allAddrs = append(allAddrs, addr)
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
				info.Managed = managedMap[addr] != ""
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

		if err := upsertManaged(body.Addr, body.RCON); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Register remote log listener on the CS2 server
		if config.C.BackendAddr != "" {
			logURL := fmt.Sprintf("http://%s:%s/internal/log", config.C.BackendAddr, config.C.Port)
			sendRCON(body.Addr, body.RCON, "log on")
			sendRCON(body.Addr, body.RCON, `logaddress_add_http "`+logURL+`"`)
			log.Printf("[gamelog] registered HTTP listener %s on %s", logURL, body.Addr)
		}

		info := query(body.Addr)
		info.Managed = true
		c.JSON(http.StatusOK, info)
	}
}

func HandleRemove() gin.HandlerFunc {
	return func(c *gin.Context) {
		addr := c.Param("addr")
		if err := removeManaged(addr); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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

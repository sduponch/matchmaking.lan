package server

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorcon/rcon"
)

// playableMapPrefixes lists the known prefixes for playable CS2 maps.
// Excludes internal/cosmetic maps (vanity_*, lobby_*, etc.).
var playableMapPrefixes = []string{"de_", "cs_", "ar_", "dm_", "gg_", "dz_", "gd_"}

var mapNameRe = regexp.MustCompile(`^[a-z]{2,3}_[a-z0-9_]+$`)

func isPlayableMap(name string) bool {
	if strings.HasSuffix(name, "_vanity") {
		return false
	}
	for _, prefix := range playableMapPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// FetchMaps sends "maps *" via RCON and returns the list of playable map names.
func FetchMaps(addr, password string) []string {
	resp, err := sendRCON(addr, password, "maps *")
	if err != nil {
		log.Printf("[rcon] %s maps fetch failed: %v", addr, err)
		return nil
	}
	var maps []string
	seen := map[string]bool{}
	for _, line := range strings.Split(resp, "\n") {
		name := strings.TrimSpace(line)
		if mapNameRe.MatchString(name) && isPlayableMap(name) && !seen[name] {
			seen[name] = true
			maps = append(maps, name)
		}
	}
	return maps
}

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

// SendRCONBatch opens a single RCON connection and sends multiple commands.
// Errors on individual commands are logged but do not abort the batch.
func SendRCONBatch(addr, password string, commands []string) error {
	if len(commands) == 0 {
		return nil
	}
	conn, err := rcon.Dial(addr, password)
	if err != nil {
		return fmt.Errorf("rcon connection failed: %w", err)
	}
	defer conn.Close()

	for _, cmd := range commands {
		if _, err := conn.Execute(cmd); err != nil {
			log.Printf("[rcon] %s command %q failed: %v", addr, cmd, err)
		}
	}
	return nil
}

func HandleChangeMap() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var body struct {
			Map string `json:"map" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		e, ok := GetByToken(token)
		if !ok || e.RCON == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "server not managed or no RCON password"})
			return
		}

		resp, err := sendRCON(e.Addr, e.RCON, "changelevel "+body.Map)
		if err != nil {
			log.Printf("[rcon] %s changelevel %s: %v", e.Addr, body.Map, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[rcon] %s changelevel %s → %q", e.Addr, body.Map, resp)
		c.JSON(http.StatusOK, gin.H{"map": body.Map})
	}
}

package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/config"
	"matchmaking.lan/backend/internal/registry"
)

var steamIDRegex = regexp.MustCompile(`https://steamcommunity\.com/openid/id/(\d+)`)

type steamPlayer struct {
	SteamID   string `json:"steamid"`
	Username  string `json:"personaname"`
	AvatarURL string `json:"avatarfull"`
}

type steamAPIResponse struct {
	Response struct {
		Players []steamPlayer `json:"players"`
	} `json:"response"`
}

// HandleCallback valide la réponse OpenID de Steam, génère un JWT
// et redirige le popup vers /auth/done?token=...
func HandleCallback(c *gin.Context) {
	if err := validateOpenID(c.Request); err != nil {
		c.String(http.StatusUnauthorized, "OpenID validation failed: %v", err)
		return
	}

	claimedID := c.Query("openid.claimed_id")
	steamID, err := extractSteamID(claimedID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid claimed_id: %v", err)
		return
	}

	player, err := fetchSteamProfile(steamID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch Steam profile: %v", err)
		return
	}

	role := "player"
	if config.C.AdminSteamIDs[steamID] {
		role = "admin"
	}

	registry.Upsert(steamID, player.Username, player.AvatarURL, role)

	token, err := GenerateJWT(steamID, player.Username, player.AvatarURL, role)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate token: %v", err)
		return
	}

	redirectURL := fmt.Sprintf("%s/auth/done?token=%s", config.C.FrontendURL, url.QueryEscape(token))
	c.Redirect(http.StatusFound, redirectURL)
}

// validateOpenID re-poste les paramètres OpenID vers Steam avec mode=check_authentication
func validateOpenID(r *http.Request) error {
	params := url.Values{}
	for key, values := range r.URL.Query() {
		params.Set(key, values[0])
	}
	params.Set("openid.mode", "check_authentication")

	resp, err := http.PostForm("https://steamcommunity.com/openid/login", params)
	if err != nil {
		return fmt.Errorf("steam request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if !strings.Contains(string(body), "is_valid:true") {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func extractSteamID(claimedID string) (string, error) {
	matches := steamIDRegex.FindStringSubmatch(claimedID)
	if len(matches) < 2 {
		return "", fmt.Errorf("no steamid in claimed_id")
	}
	return matches[1], nil
}

func fetchSteamProfile(steamID string) (*steamPlayer, error) {
	apiURL := fmt.Sprintf(
		"https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/?key=%s&steamids=%s",
		config.C.SteamAPIKey, steamID,
	)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result steamAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Response.Players) == 0 {
		return nil, fmt.Errorf("player not found")
	}

	return &result.Response.Players[0], nil
}

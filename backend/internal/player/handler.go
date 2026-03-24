package player

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"matchmaking.lan/backend/internal/bot"
	"matchmaking.lan/backend/internal/config"
)

type Profile struct {
	SteamID    string    `json:"steamid"`
	Username   string    `json:"username"`
	AvatarFull string    `json:"avatar"`
	ProfileURL string    `json:"profile_url"`
	RealName   string    `json:"real_name,omitempty"`
	Country    string    `json:"country,omitempty"`
	Status     string    `json:"status"`
	SteamLevel int       `json:"steam_level"`
	CreatedAt  time.Time `json:"created_at"`

	CS2Status    string `json:"cs2_status"`
	FaceitStatus string `json:"faceit_status"`

	PremierRating   int `json:"premier_rating"`
	CompetitiveRank int `json:"competitive_rank"`
	CompetitiveWins int `json:"competitive_wins"`

	FaceitELO       int    `json:"faceit_elo"`
	FaceitLevel     int    `json:"faceit_level"`
	FaceitNickname  string `json:"faceit_nickname,omitempty"`
	FaceitURL       string `json:"faceit_url,omitempty"`
	FaceitMatches   string `json:"faceit_matches,omitempty"`
	FaceitWinRate   string `json:"faceit_win_rate,omitempty"`
	FaceitKDRatio   string `json:"faceit_kd_ratio,omitempty"`
	FaceitHeadshots string `json:"faceit_headshots,omitempty"`
}

type CS2Response struct {
	Status          string `json:"status"`
	PremierRating   int    `json:"premier_rating"`
	CompetitiveRank int    `json:"competitive_rank"`
	CompetitiveWins int    `json:"competitive_wins"`
}

type FaceitResponse struct {
	Status    string `json:"status"`
	ELO       int    `json:"faceit_elo"`
	Level     int    `json:"faceit_level"`
	Nickname  string `json:"faceit_nickname,omitempty"`
	URL       string `json:"faceit_url,omitempty"`
	Matches   string `json:"faceit_matches,omitempty"`
	WinRate   string `json:"faceit_win_rate,omitempty"`
	KDRatio   string `json:"faceit_kd_ratio,omitempty"`
	Headshots string `json:"faceit_headshots,omitempty"`
}

func HandleGetProfile(bm *bot.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		steamID := c.Param("steamid")

		summary, err := fetchSummary(steamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		level, err := fetchLevel(steamID)
		if err != nil {
			level = 0
		}

		profile := Profile{
			SteamID:    summary.SteamID,
			Username:   summary.PersonaName,
			AvatarFull: summary.AvatarFull,
			ProfileURL: summary.ProfileURL,
			RealName:   summary.RealName,
			Country:    summary.LocCountryCode,
			Status:     personaState(summary.PersonaState),
			SteamLevel: level,
			CreatedAt:  time.Unix(int64(summary.TimeCreated), 0),
		}

		if entry, ok := getCS2Cached(steamID); ok {
			profile.CS2Status = entry.status
			if entry.data != nil {
				profile.PremierRating = entry.data.PremierRating
				profile.CompetitiveRank = entry.data.CompetitiveRank
				profile.CompetitiveWins = entry.data.CompetitiveWins
			}
		} else {
			profile.CS2Status = CS2StatusRetrieving
			triggerCS2Fetch(steamID, bm)
		}

		if config.C.FaceitAPIKey == "" {
			profile.FaceitStatus = FaceitStatusUnavailable
		} else if entry, ok := getFaceitCached(steamID); ok {
			profile.FaceitStatus = entry.status
			if entry.data != nil {
				f := entry.data
				profile.FaceitELO = f.ELO()
				profile.FaceitLevel = f.Level()
				profile.FaceitNickname = f.Nickname
				profile.FaceitURL = f.FaceitURL
				profile.FaceitMatches = f.Matches
				profile.FaceitWinRate = f.WinRate
				profile.FaceitKDRatio = f.KDRatio
				profile.FaceitHeadshots = f.Headshots
			}
		} else {
			profile.FaceitStatus = FaceitStatusRetrieving
			triggerFaceitFetch(steamID)
		}

		c.JSON(http.StatusOK, profile)
	}
}

func HandleGetCS2(bm *bot.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		steamID := c.Param("steamid")

		entry, ok := getCS2Cached(steamID)
		if !ok {
			triggerCS2Fetch(steamID, bm)
			c.JSON(http.StatusOK, CS2Response{Status: CS2StatusRetrieving})
			return
		}

		resp := CS2Response{Status: entry.status}
		if entry.data != nil {
			resp.PremierRating = entry.data.PremierRating
			resp.CompetitiveRank = entry.data.CompetitiveRank
			resp.CompetitiveWins = entry.data.CompetitiveWins
		}
		c.JSON(http.StatusOK, resp)
	}
}

func HandleGetFaceit() gin.HandlerFunc {
	return func(c *gin.Context) {
		steamID := c.Param("steamid")

		if config.C.FaceitAPIKey == "" {
			c.JSON(http.StatusOK, FaceitResponse{Status: FaceitStatusUnavailable})
			return
		}

		entry, ok := getFaceitCached(steamID)
		if !ok {
			triggerFaceitFetch(steamID)
			c.JSON(http.StatusOK, FaceitResponse{Status: FaceitStatusRetrieving})
			return
		}

		resp := FaceitResponse{Status: entry.status}
		if entry.data != nil {
			f := entry.data
			resp.ELO = f.ELO()
			resp.Level = f.Level()
			resp.Nickname = f.Nickname
			resp.URL = f.FaceitURL
			resp.Matches = f.Matches
			resp.WinRate = f.WinRate
			resp.KDRatio = f.KDRatio
			resp.Headshots = f.Headshots
		}
		c.JSON(http.StatusOK, resp)
	}
}

type steamSummary struct {
	SteamID        string `json:"steamid"`
	PersonaName    string `json:"personaname"`
	AvatarFull     string `json:"avatarfull"`
	ProfileURL     string `json:"profileurl"`
	RealName       string `json:"realname"`
	LocCountryCode string `json:"loccountrycode"`
	PersonaState   int    `json:"personastate"`
	TimeCreated    int    `json:"timecreated"`
}

func fetchSummary(steamID string) (*steamSummary, error) {
	url := fmt.Sprintf(
		"https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/?key=%s&steamids=%s",
		config.C.SteamAPIKey, steamID,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			Players []steamSummary `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Response.Players) == 0 {
		return nil, fmt.Errorf("player not found")
	}
	return &result.Response.Players[0], nil
}

func fetchLevel(steamID string) (int, error) {
	url := fmt.Sprintf(
		"https://api.steampowered.com/IPlayerService/GetSteamLevel/v1/?key=%s&steamid=%s",
		config.C.SteamAPIKey, steamID,
	)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			PlayerLevel int `json:"player_level"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Response.PlayerLevel, nil
}

func personaState(state int) string {
	switch state {
	case 1:
		return "online"
	case 2:
		return "busy"
	case 3:
		return "away"
	case 4:
		return "snooze"
	default:
		return "offline"
	}
}

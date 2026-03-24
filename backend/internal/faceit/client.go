package faceit

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"matchmaking.lan/backend/internal/config"
)

const baseURL = "https://open.faceit.com/data/v4"

var httpClient = &http.Client{Timeout: 5 * time.Second}

type gameInfo struct {
	ELO   int `json:"faceit_elo"`
	Level int `json:"skill_level"`
}

type PlayerInfo struct {
	FaceitID  string              `json:"player_id"`
	Nickname  string              `json:"nickname"`
	Avatar    string              `json:"avatar"`
	FaceitURL string              `json:"faceit_url"`
	Games     map[string]gameInfo `json:"games"`
}

func (p PlayerInfo) ELO() int {
	return p.Games["cs2"].ELO
}

func (p PlayerInfo) Level() int {
	return p.Games["cs2"].Level
}

type PlayerStats struct {
	Matches    string `json:"Matches"`
	Wins       string `json:"Wins"`
	WinRate    string `json:"Win Rate %"`
	KDRatio    string `json:"Average K/D Ratio"`
	Headshots  string `json:"Average Headshots %"`
}

type Stats struct {
	PlayerInfo
	PlayerStats
}

func get(path string, out any) error {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.C.FaceitAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("faceit API returned %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func GetPlayerBySteamID(steamID string) (*Stats, error) {
	var info PlayerInfo
	if err := get(fmt.Sprintf("/players?game=cs2&game_player_id=%s", steamID), &info); err != nil {
		log.Printf("[faceit] player lookup failed for %s: %v", steamID, err)
		return nil, err
	}
	log.Printf("[faceit] player found: %s (id=%s, elo=%d, level=%d)", info.Nickname, info.FaceitID, info.ELO(), info.Level())

	var statsResp struct {
		Lifetime PlayerStats `json:"lifetime"`
	}
	gameID := "cs2"
	if err := get(fmt.Sprintf("/players/%s/stats/cs2", info.FaceitID), &statsResp); err != nil {
		gameID = "csgo"
		if err2 := get(fmt.Sprintf("/players/%s/stats/csgo", info.FaceitID), &statsResp); err2 != nil {
			log.Printf("[faceit] stats unavailable for %s: %v", info.Nickname, err2)
			return &Stats{PlayerInfo: info}, nil
		}
	}
	log.Printf("[faceit] stats for %s (%s): matches=%s winrate=%s kd=%s hs=%s",
		info.Nickname, gameID, statsResp.Lifetime.Matches, statsResp.Lifetime.WinRate,
		statsResp.Lifetime.KDRatio, statsResp.Lifetime.Headshots,
	)

	return &Stats{
		PlayerInfo:  info,
		PlayerStats: statsResp.Lifetime,
	}, nil
}

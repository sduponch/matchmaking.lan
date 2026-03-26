package registry

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

const storeFile = "players.json"

type Player struct {
	SteamID  string    `json:"steamid"`
	Username string    `json:"username"`
	Avatar   string    `json:"avatar"`
	Role     string    `json:"role"`
	Team     string    `json:"team,omitempty"`
	LastSeen time.Time `json:"last_seen"`
}

var (
	mu      sync.RWMutex
	players map[string]*Player
)

func init() {
	load()
}

func load() {
	data, err := os.ReadFile(storeFile)
	if err != nil {
		players = map[string]*Player{}
		return
	}
	if err := json.Unmarshal(data, &players); err != nil {
		players = map[string]*Player{}
	}
}

func save() error {
	data, err := json.MarshalIndent(players, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storeFile, data, 0644)
}

// Upsert creates or updates a player entry. Called on each successful login.
func Upsert(steamid, username, avatar, role string) {
	mu.Lock()
	defer mu.Unlock()
	team := ""
	if existing, ok := players[steamid]; ok {
		team = existing.Team
	}
	players[steamid] = &Player{
		SteamID:  steamid,
		Username: username,
		Avatar:   avatar,
		Role:     role,
		Team:     team,
		LastSeen: time.Now(),
	}
	_ = save()
}

// SyncRoles updates the role of all registered players based on the current admin list.
// Should be called at startup after config is loaded.
func SyncRoles(adminIDs map[string]bool) {
	mu.Lock()
	defer mu.Unlock()
	changed := false
	for id, p := range players {
		newRole := "player"
		if adminIDs[id] {
			newRole = "admin"
		}
		if p.Role != newRole {
			players[id].Role = newRole
			changed = true
		}
	}
	if changed {
		_ = save()
	}
}

// SetTeam updates the team field for a player.
func SetTeam(steamid, team string) {
	mu.Lock()
	defer mu.Unlock()
	if p, ok := players[steamid]; ok {
		p.Team = team
		_ = save()
	}
}

// List returns all players sorted by last seen (most recent first).
func List() []*Player {
	mu.RLock()
	defer mu.RUnlock()
	list := make([]*Player, 0, len(players))
	for _, p := range players {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastSeen.After(list[j].LastSeen)
	})
	return list
}

package teams

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

const storeFile = "teams.json"

type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Players   []string  `json:"players"` // steamids
	CreatedAt time.Time `json:"created_at"`
}

var (
	mu    sync.RWMutex
	store map[string]*Team
)

func init() {
	load()
}

func load() {
	data, err := os.ReadFile(storeFile)
	if err != nil {
		store = map[string]*Team{}
		return
	}
	if err := json.Unmarshal(data, &store); err != nil {
		store = map[string]*Team{}
	}
}

func save() error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storeFile, data, 0644)
}

func newID() string {
	return fmt.Sprintf("%08x", rand.Uint32())
}

func Create(name string) (*Team, error) {
	mu.Lock()
	defer mu.Unlock()
	t := &Team{ID: newID(), Name: name, Players: []string{}, CreatedAt: time.Now()}
	store[t.ID] = t
	return t, save()
}

func Delete(id string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := store[id]; !ok {
		return fmt.Errorf("team not found")
	}
	delete(store, id)
	return save()
}

func Get(id string) (*Team, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := store[id]
	return t, ok
}

func List() []*Team {
	mu.RLock()
	defer mu.RUnlock()
	list := make([]*Team, 0, len(store))
	for _, t := range store {
		list = append(list, t)
	}
	return list
}

func AddPlayer(teamID, steamid string) error {
	mu.Lock()
	defer mu.Unlock()
	t, ok := store[teamID]
	if !ok {
		return fmt.Errorf("team not found")
	}
	for _, s := range t.Players {
		if s == steamid {
			return nil // already in team
		}
	}
	t.Players = append(t.Players, steamid)
	return save()
}

func RemovePlayer(teamID, steamid string) error {
	mu.Lock()
	defer mu.Unlock()
	t, ok := store[teamID]
	if !ok {
		return fmt.Errorf("team not found")
	}
	for i, s := range t.Players {
		if s == steamid {
			t.Players = append(t.Players[:i], t.Players[i+1:]...)
			return save()
		}
	}
	return nil
}

package mappoolconfig

import (
	"encoding/json"
	"os"
	"sync"
)

const dataFile = "map_pool.json"

// Pool maps a mode prefix (e.g. "de_") to its list of official maps.
type Pool map[string][]string

var (
	mu   sync.RWMutex
	pool Pool
)

var defaultPool = Pool{
	"de_": {"de_ancient", "de_anubis", "de_dust2", "de_inferno", "de_mirage", "de_nuke", "de_overpass", "de_train", "de_vertigo"},
	"cs_": {"cs_italy", "cs_office"},
	"ar_": {"ar_baggage", "ar_shoots"},
	"dm_": {"dm_rust"},
}

func init() {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		pool = defaultPool
		_ = save()
		return
	}
	if err := json.Unmarshal(data, &pool); err != nil {
		pool = defaultPool
	}
}

func save() error {
	data, err := json.MarshalIndent(pool, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0644)
}

// Get returns a copy of the full pool.
func Get() Pool {
	mu.RLock()
	defer mu.RUnlock()
	out := make(Pool, len(pool))
	for k, v := range pool {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Set replaces the full pool and persists it.
func Set(p Pool) error {
	mu.Lock()
	defer mu.Unlock()
	pool = p
	return save()
}

// ForPrefix returns the official maps for a given prefix.
func ForPrefix(prefix string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cp := make([]string, len(pool[prefix]))
	copy(cp, pool[prefix])
	return cp
}

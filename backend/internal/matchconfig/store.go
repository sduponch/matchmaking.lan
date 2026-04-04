package matchconfig

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

const profilesFile = "match_profiles.json"

// Phases lists all supported match phases in order.
var Phases = []string{"warmup", "knife", "live", "halftime", "game_over"}

type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Tags      []string  `json:"tags,omitempty"` // game modes this profile applies to; empty = all
	CreatedAt time.Time `json:"created_at"`
}

var (
	mu       sync.RWMutex
	profiles map[string]*Profile
)

func init() {
	load()
	seedServerInitCFG()
	seedDefaultProfile()
}

func load() {
	profiles = map[string]*Profile{}
	data, err := os.ReadFile(profilesFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &profiles)
}

func save() error {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profilesFile, data, 0644)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// List returns all profiles sorted by creation date.
func List() []*Profile {
	mu.RLock()
	defer mu.RUnlock()
	list := make([]*Profile, 0, len(profiles))
	for _, p := range profiles {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list
}

// Get returns a profile by ID.
func Get(id string) (*Profile, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := profiles[id]
	return p, ok
}

// Create adds a new profile and returns it with its generated ID.
func Create(p *Profile) error {
	mu.Lock()
	defer mu.Unlock()
	p.ID = newID()
	p.CreatedAt = time.Now()
	profiles[p.ID] = p
	return save()
}

// Update replaces profile metadata (preserves ID and CreatedAt).
func Update(id string, update *Profile) error {
	mu.Lock()
	defer mu.Unlock()
	existing, ok := profiles[id]
	if !ok {
		return os.ErrNotExist
	}
	update.ID = id
	update.CreatedAt = existing.CreatedAt
	profiles[id] = update
	return save()
}

// Delete removes a profile and its CFG files.
func Delete(id string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := profiles[id]; !ok {
		return os.ErrNotExist
	}
	delete(profiles, id)
	_ = DeleteProfileCFGs(id)
	return save()
}

func seedDefaultProfile() {
	if len(profiles) > 0 {
		return
	}
	id := "default_5v5"
	profiles[id] = &Profile{
		ID:        id,
		Name:      "5v5 Compétitif",
		CreatedAt: time.Now(),
	}
	_ = save()
	_ = SetCFG(id, "warmup", defaultWarmupCFG)
	_ = SetCFG(id, "knife", defaultKnifeCFG)
	_ = SetCFG(id, "live", defaultLiveCFG)
}

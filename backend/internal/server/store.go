package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

const storeFile = "servers.json"

type serverEntry struct {
	RCON  string `json:"rcon"`
	Token string `json:"token"`
}

var (
	mu        sync.RWMutex
	managed   map[string]*serverEntry // addr → entry
	lastLogMu sync.RWMutex
	lastLogs  = map[string]time.Time{} // addr → last log received (in-memory only)
)

func init() {
	load()
}

func load() {
	managed = map[string]*serverEntry{}
	data, err := os.ReadFile(storeFile)
	if err != nil {
		return
	}
	// Try new format first
	if err := json.Unmarshal(data, &managed); err == nil {
		return
	}
	// Migrate from old format (map[string]string addr→rcon)
	var old map[string]string
	if err := json.Unmarshal(data, &old); err == nil {
		for addr, rcon := range old {
			managed[addr] = &serverEntry{RCON: rcon, Token: newToken()}
		}
		_ = save()
	}
}

func save() error {
	data, err := json.MarshalIndent(managed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storeFile, data, 0644)
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// getManagedAddrs returns addr→rcon for all managed servers.
func getManagedAddrs() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	cp := make(map[string]string, len(managed))
	for addr, e := range managed {
		cp[addr] = e.RCON
	}
	return cp
}

// getToken returns the log token for a given server address.
func getToken(addr string) string {
	mu.RLock()
	defer mu.RUnlock()
	if e, ok := managed[addr]; ok {
		return e.Token
	}
	return ""
}

// UpdateLastLog records the current time as the last log reception for addr (in memory only).
func UpdateLastLog(addr string) {
	lastLogMu.Lock()
	lastLogs[addr] = time.Now()
	lastLogMu.Unlock()
}

// GetLastLogAt returns the last log reception time for addr, or nil if never received.
func GetLastLogAt(addr string) *time.Time {
	lastLogMu.RLock()
	t, ok := lastLogs[addr]
	lastLogMu.RUnlock()
	if !ok {
		return nil
	}
	return &t
}

// GetAddrByToken resolves a log token back to a server address.
func GetAddrByToken(token string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for addr, e := range managed {
		if e.Token == token {
			return addr, true
		}
	}
	return "", false
}

func upsertManaged(addr, rcon string) error {
	mu.Lock()
	defer mu.Unlock()
	if e, ok := managed[addr]; ok {
		e.RCON = rcon
	} else {
		managed[addr] = &serverEntry{RCON: rcon, Token: newToken()}
	}
	return save()
}

func removeManaged(addr string) error {
	mu.Lock()
	defer mu.Unlock()
	delete(managed, addr)
	return save()
}

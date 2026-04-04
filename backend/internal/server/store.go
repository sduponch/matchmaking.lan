package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"

	"matchmaking.lan/backend/internal/config"
)

const storeFile = "servers.json"

type serverEntry struct {
	Addr  string   `json:"addr"`
	Name  string   `json:"name"`
	RCON  string   `json:"rcon"`
	Token string   `json:"token"`
	Maps  []string `json:"maps,omitempty"`
}

var (
	mu        sync.RWMutex
	managed   map[string]*serverEntry // token → entry
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

	// Try current format: map[token]*serverEntry (addr inside entry)
	var byToken map[string]*serverEntry
	if err := json.Unmarshal(data, &byToken); err == nil {
		// Validate: entries must have an Addr field (distinguishes from old formats)
		valid := true
		for _, e := range byToken {
			if e.Addr == "" {
				valid = false
				break
			}
		}
		if valid && len(byToken) > 0 {
			managed = byToken
			return
		}
	}

	// Migrate from v2 format: map[addr]{rcon, token}
	var v2 map[string]*struct {
		RCON  string `json:"rcon"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &v2); err == nil {
		allHaveToken := true
		for _, e := range v2 {
			if e.Token == "" {
				allHaveToken = false
				break
			}
		}
		if allHaveToken && len(v2) > 0 {
			for addr, e := range v2 {
				managed[e.Token] = &serverEntry{Addr: addr, RCON: e.RCON, Token: e.Token}
			}
			_ = save()
			return
		}
	}

	// Migrate from v1 format: map[addr]string (rcon only)
	var v1 map[string]string
	if err := json.Unmarshal(data, &v1); err == nil {
		for addr, rcon := range v1 {
			tok := newToken()
			managed[tok] = &serverEntry{Addr: addr, RCON: rcon, Token: tok}
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

// GetAll returns a snapshot of all managed servers (token → entry).
func GetAll() map[string]*serverEntry {
	mu.RLock()
	defer mu.RUnlock()
	cp := make(map[string]*serverEntry, len(managed))
	for tok, e := range managed {
		cp[tok] = e
	}
	return cp
}

// GetByToken returns the server entry for a given token.
func GetByToken(token string) (*serverEntry, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := managed[token]
	return e, ok
}

// GetAddrRCON returns the address and RCON password for a server by token.
// ok is false if the token is unknown or has no RCON configured.
func GetAddrRCON(token string) (addr, rcon string, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, exists := managed[token]
	if !exists {
		return "", "", false
	}
	return e.Addr, e.RCON, e.RCON != ""
}

// GetTokenByAddr resolves a server address back to its token.
func GetTokenByAddr(addr string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for tok, e := range managed {
		if e.Addr == addr {
			return tok, true
		}
	}
	return "", false
}

// GetLogSetupCmds returns the RCON commands to (re-)register the log endpoint for a server.
// Should be sent before changelevel and again on warmup.start to survive map reloads.
func GetLogSetupCmds(token string) []string {
	if config.C.BackendURL == "" {
		return nil
	}
	logURL := config.C.BackendURL + "/internal/log/" + token
	return []string{
		"log on",
		"logaddress_delall_http",
		`logaddress_add_http "` + logURL + `"`,
	}
}

// GetAddrByToken resolves a log token back to a server address.
func GetAddrByToken(token string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if e, ok := managed[token]; ok {
		return e.Addr, true
	}
	return "", false
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

// upsertManaged adds or updates a server entry keyed by token.
func upsertManaged(addr, rcon string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	// Check if addr already exists → reuse its token
	for tok, e := range managed {
		if e.Addr == addr {
			e.RCON = rcon
			return tok, save()
		}
	}
	tok := newToken()
	managed[tok] = &serverEntry{Addr: addr, RCON: rcon, Token: tok}
	return tok, save()
}

// removeManaged removes a server by token.
func removeManaged(token string) error {
	mu.Lock()
	defer mu.Unlock()
	delete(managed, token)
	return save()
}

// setMaps stores the available map list for a server.
func setMaps(token string, maps []string) {
	mu.Lock()
	defer mu.Unlock()
	if e, ok := managed[token]; ok {
		e.Maps = maps
		_ = save()
	}
}

// setName updates the display name of a server.
func setName(token, name string) error {
	mu.Lock()
	defer mu.Unlock()
	e, ok := managed[token]
	if !ok {
		return os.ErrNotExist
	}
	e.Name = name
	return save()
}

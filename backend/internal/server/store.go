package server

import (
	"encoding/json"
	"os"
	"sync"
)

const storeFile = "servers.json"

var (
	mu      sync.RWMutex
	managed map[string]string // addr → rcon password
)

func init() {
	load()
}

func load() {
	data, err := os.ReadFile(storeFile)
	if err != nil {
		managed = map[string]string{}
		return
	}
	if err := json.Unmarshal(data, &managed); err != nil {
		managed = map[string]string{}
	}
}

func save() error {
	data, err := json.Marshal(managed)
	if err != nil {
		return err
	}
	return os.WriteFile(storeFile, data, 0644)
}

func getManagedAddrs() map[string]string {
	mu.RLock()
	defer mu.RUnlock()
	cp := make(map[string]string, len(managed))
	for k, v := range managed {
		cp[k] = v
	}
	return cp
}

func upsertManaged(addr, rcon string) error {
	mu.Lock()
	defer mu.Unlock()
	managed[addr] = rcon
	return save()
}

func removeManaged(addr string) error {
	mu.Lock()
	defer mu.Unlock()
	delete(managed, addr)
	return save()
}

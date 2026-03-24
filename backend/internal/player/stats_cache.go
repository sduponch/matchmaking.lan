package player

import (
	"log"
	"strings"
	"sync"
	"time"

	"matchmaking.lan/backend/internal/bot"
	"matchmaking.lan/backend/internal/config"
	"matchmaking.lan/backend/internal/faceit"
)

// États CS2
const (
	CS2StatusRetrieving   = "retrieving"
	CS2StatusReady        = "ready"
	CS2StatusPendingInvite = "pending_invite"
	CS2StatusUnavailable  = "unavailable"
)

// États Faceit
const (
	FaceitStatusRetrieving = "retrieving"
	FaceitStatusReady      = "ready"
	FaceitStatusNotFound   = "not_found"
	FaceitStatusUnavailable = "unavailable"
)

var cs2TTL = map[string]time.Duration{
	CS2StatusReady:         5 * time.Minute,
	CS2StatusPendingInvite: 2 * time.Minute,
	CS2StatusUnavailable:   30 * time.Second,
}

var faceitTTL = map[string]time.Duration{
	FaceitStatusReady:       5 * time.Minute,
	FaceitStatusNotFound:    10 * time.Minute,
	FaceitStatusUnavailable: 30 * time.Second,
}

// --- CS2 cache ---

type cs2Entry struct {
	status string
	data   *bot.RankInfo
	ts     time.Time
}

var (
	cs2Cache sync.Map
	cs2Lock  sync.Map
)

func getCS2Cached(steamID string) (*cs2Entry, bool) {
	v, ok := cs2Cache.Load(steamID)
	if !ok {
		return nil, false
	}
	e := v.(*cs2Entry)
	ttl := cs2TTL[e.status]
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	if time.Since(e.ts) > ttl {
		cs2Cache.Delete(steamID)
		return nil, false
	}
	return e, true
}

func triggerCS2Fetch(steamID string, bm *bot.Manager) {
	if _, loaded := cs2Lock.LoadOrStore(steamID, struct{}{}); loaded {
		return
	}
	go func() {
		defer cs2Lock.Delete(steamID)

		rank, err := bm.GetRank(steamID)
		if err != nil {
			status := CS2StatusUnavailable
			if strings.Contains(err.Error(), "timeout") {
				status = CS2StatusPendingInvite
			}
			log.Printf("[cs2] %s for %s: %v", status, steamID, err)
			cs2Cache.Store(steamID, &cs2Entry{status: status, ts: time.Now()})
			return
		}

		cs2Cache.Store(steamID, &cs2Entry{status: CS2StatusReady, data: rank, ts: time.Now()})
		log.Printf("[cs2] ready for %s", steamID)
	}()
}

// --- Faceit cache ---

type faceitEntry struct {
	status string
	data   *faceit.Stats
	ts     time.Time
}

var (
	faceitCache sync.Map
	faceitLock  sync.Map
)

func getFaceitCached(steamID string) (*faceitEntry, bool) {
	v, ok := faceitCache.Load(steamID)
	if !ok {
		return nil, false
	}
	e := v.(*faceitEntry)
	ttl := faceitTTL[e.status]
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	if time.Since(e.ts) > ttl {
		faceitCache.Delete(steamID)
		return nil, false
	}
	return e, true
}

func triggerFaceitFetch(steamID string) {
	if config.C.FaceitAPIKey == "" {
		return
	}
	if _, loaded := faceitLock.LoadOrStore(steamID, struct{}{}); loaded {
		return
	}
	go func() {
		defer faceitLock.Delete(steamID)

		f, err := faceit.GetPlayerBySteamID(steamID)
		if err != nil {
			status := FaceitStatusUnavailable
			if strings.Contains(err.Error(), "not found") {
				status = FaceitStatusNotFound
			}
			log.Printf("[faceit] %s for %s: %v", status, steamID, err)
			faceitCache.Store(steamID, &faceitEntry{status: status, ts: time.Now()})
			return
		}

		faceitCache.Store(steamID, &faceitEntry{status: FaceitStatusReady, data: f, ts: time.Now()})
		log.Printf("[faceit] ready for %s", steamID)
	}()
}

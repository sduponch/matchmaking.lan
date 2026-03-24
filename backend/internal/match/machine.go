package match

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"matchmaking.lan/backend/internal/gamelog"
)

type Machine struct {
	mu    sync.RWMutex
	state MatchState
}

func newMachine() *Machine {
	return &Machine{
		state: MatchState{
			Phase:   PhaseIdle,
			Players: map[string]*PlayerStat{},
		},
	}
}

func (m *Machine) State() MatchState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := m.state
	cp.Players = make(map[string]*PlayerStat, len(m.state.Players))
	for k, v := range m.state.Players {
		p := *v
		cp.Players[k] = &p
	}
	return cp
}

func (m *Machine) Apply(e *gamelog.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &m.state

	switch e.Type {

	case "cs2.game.commencing":
		now := time.Now()
		s.Phase = PhaseWarmup
		s.Round = 0
		s.ScoreCT = 0
		s.ScoreT = 0
		s.StartedAt = &now
		s.Players = map[string]*PlayerStat{}
		log.Printf("[match] %s → warmup (game commencing)", e.Server)

	case "cs2.warmup.start":
		s.Phase = PhaseWarmup
		log.Printf("[match] %s → warmup", e.Server)

	case "cs2.warmup.end":
		log.Printf("[match] %s warmup ended", e.Server)

	case "cs2.match.start":
		now := time.Now()
		s.Phase = PhaseLive
		s.Round = 0
		s.ScoreCT = 0
		s.ScoreT = 0
		s.StartedAt = &now
		if mp := e.Fields["map"]; mp != "" {
			s.Map = mp
		}
		log.Printf("[match] %s → live  map=%s", e.Server, s.Map)

	case "cs2.round.start":
		if s.Phase == PhaseIdle || s.Phase == PhaseWarmup {
			s.Phase = PhaseLive
			if s.StartedAt == nil {
				now := time.Now()
				s.StartedAt = &now
			}
		}
		s.Round++
		log.Printf("[match] %s round %d start (phase=%s)", e.Server, s.Round, s.Phase)

	case "cs2.round.end":
		log.Printf("[match] %s round %d end  CT=%d T=%d", e.Server, s.Round, s.ScoreCT, s.ScoreT)

	case "cs2.score":
		score := atoi(e.Fields["score"])
		switch e.Fields["team"] {
		case "CT":
			s.ScoreCT = score
		case "TERRORIST":
			s.ScoreT = score
		}
		log.Printf("[match] %s score CT=%d T=%d", e.Server, s.ScoreCT, s.ScoreT)

	case "cs2.match.status":
		// Authoritative score/round sync from server
		if rounds := atoi(e.Fields["rounds"]); rounds >= 0 {
			s.Round = rounds
		}
		s.ScoreCT = atoi(e.Fields["score_ct"])
		s.ScoreT = atoi(e.Fields["score_t"])
		if mp := e.Fields["map"]; mp != "" {
			s.Map = mp
		}
		log.Printf("[match] %s status sync round=%d CT=%d T=%d map=%s",
			e.Server, s.Round, s.ScoreCT, s.ScoreT, s.Map)

	case "cs2.kill", "cs2.kill.headshot":
		if p := m.getOrCreatePlayer(e.Fields["killer"], e.Fields["killer_steamid"], e.Fields["killer_team"]); p != nil {
			p.Kills++
		}
		if p := m.getOrCreatePlayer(e.Fields["victim"], e.Fields["victim_steamid"], e.Fields["victim_team"]); p != nil {
			p.Deaths++
		}

	case "cs2.kill.bomb", "cs2.kill.suicide":
		if p := m.getOrCreatePlayer(e.Fields["victim"], e.Fields["victim_steamid"], e.Fields["victim_team"]); p != nil {
			p.Deaths++
		}

	case "cs2.round.stats":
		rs, ok := e.Extra.(*gamelog.RoundStats)
		if !ok {
			break
		}
		// Update map from round stats
		if rs.Map != "" {
			s.Map = rs.Map
		}
		// Merge detailed per-player stats matched by account ID
		for _, rp := range rs.Players {
			if rp.AccountID == "" {
				continue
			}
			if p := m.findByAccountID(rp.AccountID); p != nil {
				p.Kills = rp.Kills
				p.Deaths = rp.Deaths
				p.Assists = rp.Assists
				p.Damage = rp.Damage
				p.HSP = rp.HSP
				p.ADR = rp.ADR
				p.Money = rp.Money
				p.MVP = rp.MVP
			}
		}
		log.Printf("[match] %s round stats round=%d CT=%d T=%d players=%d",
			e.Server, rs.Round, rs.ScoreCT, rs.ScoreT, len(rs.Players))

	case "cs2.player.connect":
		m.getOrCreatePlayer(e.Fields["player"], e.Fields["player_steamid"], e.Fields["player_team"])
		log.Printf("[match] %s player joined: %s", e.Server, e.Fields["player"])

	case "cs2.player.enter":
		m.getOrCreatePlayer(e.Fields["player"], e.Fields["player_steamid"], e.Fields["player_team"])

	case "cs2.player.switch":
		// player_nt token: no team field; new team is in "to"
		if p := m.getOrCreatePlayer(e.Fields["player"], e.Fields["player_steamid"], e.Fields["to"]); p != nil {
			p.Team = e.Fields["to"]
			log.Printf("[match] %s player %s switched to %s", e.Server, e.Fields["player"], e.Fields["to"])
		}

	case "cs2.player.disconnect":
		log.Printf("[match] %s player left:   %s", e.Server, e.Fields["player"])

	case "cs2.game.over":
		s.Phase = PhaseGameOver
		log.Printf("[match] %s → game over  CT=%d T=%d", e.Server, s.ScoreCT, s.ScoreT)
	}
}

func (m *Machine) getOrCreatePlayer(name, steamid, team string) *PlayerStat {
	if steamid == "" {
		return nil
	}
	key := steam3ToSteam64(steamid)
	if p, ok := m.state.Players[key]; ok {
		if team != "" {
			p.Team = team
		}
		return p
	}
	p := &PlayerStat{Name: name, SteamID: key, Team: team}
	m.state.Players[key] = p
	return p
}

// findByAccountID converts the 32-bit CS2 accountid to Steam64 and looks up directly.
// steam64 = 76561197960265728 + accountid
func (m *Machine) findByAccountID(accountID string) *PlayerStat {
	id, err := strconv.ParseInt(strings.TrimSpace(accountID), 10, 64)
	if err != nil || id == 0 {
		return nil
	}
	steam64 := strconv.FormatInt(76561197960265728+id, 10)
	return m.state.Players[steam64]
}

// steam3ToSteam64 converts "[U:1:160633]" → "76561197960426361".
// Returns the input unchanged if it is not in Steam3 format.
func steam3ToSteam64(steamid string) string {
	if strings.HasPrefix(steamid, "[U:1:") && strings.HasSuffix(steamid, "]") {
		inner := steamid[5 : len(steamid)-1]
		if id, err := strconv.ParseInt(inner, 10, 64); err == nil {
			return strconv.FormatInt(76561197960265728+id, 10)
		}
	}
	return steamid
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

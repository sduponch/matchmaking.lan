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
	mu               sync.RWMutex
	state            MatchState
	expectWarmup     bool           // armed by ExpectWarmup() when encounter.Start sends changelevel; consumed by cs2.map.started
	halftimeNotified bool           // true once OnPhaseChange("halftime") fired this map
	inKnifeSetup     bool           // true between !ready→knife and knife cs2.match.start
	knifeOver        bool           // true after cs2.game.over in PhaseKnife — waiting for !ct/!t
	knifeWinSide     string         // "CT" or "TERRORIST" — winner of the knife round
	readySet         map[string]bool // steamid → true, tracks !ready during warmup
	firstPlayerDone  bool           // true once OnFirstPlayerJoin has fired for this warmup period
}

func newMachine() *Machine {
	return &Machine{
		state: MatchState{
			Phase:   PhaseIdle,
			Players: map[string]*PlayerStat{},
		},
		readySet: map[string]bool{},
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

	case "cs2.map.loading":
		// Full state reset — changelevel starts. Clears all match state and flags.
		newMap := e.Fields["map"]
		*s = MatchState{
			Map:     newMap,
			Phase:   PhaseIdle,
			Players: map[string]*PlayerStat{},
		}
		m.halftimeNotified = false
		m.inKnifeSetup = false
		m.knifeOver = false
		m.knifeWinSide = ""
		m.readySet = map[string]bool{}
		m.firstPlayerDone = false
		log.Printf("[match] %s map loading → %s (state reset)", e.Server, newMap)

	case "cs2.map.started":
		// Fires on map load — also on bot-kick cycles, so only push warmup CFG when
		// ExpectWarmup() was explicitly called (i.e. an encounter just sent changelevel).
		path := e.Fields["path"]
		mapName := path
		if i := strings.Index(path, "+"); i != -1 {
			mapName = path[:i]
		}
		if mapName != "" {
			s.Map = mapName
		}
		s.Phase = PhaseWarmup
		log.Printf("[match] %s map started → %s (expectWarmup=%v)", e.Server, mapName, m.expectWarmup)
		if m.expectWarmup && OnPhaseChange != nil {
			m.expectWarmup = false
			go OnPhaseChange(e.Server, "warmup")
		}

	case "cs2.game.commencing":
		// Game restart (bot-kick cycle, mp_restartgame) — partial reset, no CFG push.
		now := time.Now()
		m.halftimeNotified = false
		m.knifeOver = false
		m.knifeWinSide = ""
		m.readySet = map[string]bool{}
		m.firstPlayerDone = false
		s.Phase = PhaseWarmup
		s.Round = 0
		s.ScoreCT = 0
		s.ScoreT = 0
		s.StartedAt = &now
		s.Players = map[string]*PlayerStat{}
		log.Printf("[match] %s → warmup (game commencing)", e.Server)

	case "cs2.warmup.start":
		// State update only — CFG push is handled by cs2.map.started.
		s.Phase = PhaseWarmup
		m.readySet = map[string]bool{}
		log.Printf("[match] %s → warmup", e.Server)

	case "cs2.warmup.end":
		log.Printf("[match] %s warmup ended", e.Server)
		if OnPhaseChange != nil {
			go OnPhaseChange(e.Server, "warmup_end")
		}

	case "cs2.match.start":
		prevPhase := s.Phase
		m.inKnifeSetup = false // knife setup complete, match is live
		if mp := e.Fields["map"]; mp != "" {
			s.Map = mp
		}
		if prevPhase == PhaseKnife {
			// Knife match has started — stay in PhaseKnife for round tracking.
			log.Printf("[match] %s knife match started map=%s", e.Server, s.Map)
			break
		}
		now := time.Now()
		s.Phase = PhaseLive
		s.Round = 0
		s.ScoreCT = 0
		s.ScoreT = 0
		s.StartedAt = &now
		log.Printf("[match] %s → live  map=%s (prevPhase=%s)", e.Server, s.Map, prevPhase)
		// Second half starts after halftime — update hostname only (live.cfg already active).
		if OnPhaseChange != nil && prevPhase == PhaseHalfTime {
			go OnPhaseChange(e.Server, "second_half")
		}
		// NOTE: live.cfg is pushed via !ready → OnPhaseChange("first_half") or OnKnifeChoice,
		// never from here. CS2 fires match.start on initial map load (state dump) which is
		// not a real match start — pushing live.cfg here would race with warmup setup.

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
		// Halftime detection: fire once when total rounds == half of maxRounds.
		if s.Phase == PhaseLive && !m.halftimeNotified {
			maxRounds := 24
			if GetEncounterInfo != nil {
				if _, _, mr, ok := GetEncounterInfo(e.Server); ok && mr > 0 {
					maxRounds = mr
				}
			}
			if s.ScoreCT+s.ScoreT == maxRounds/2 {
				m.halftimeNotified = true
				s.Phase = PhaseHalfTime
				log.Printf("[match] %s → halftime  CT=%d T=%d", e.Server, s.ScoreCT, s.ScoreT)
				if OnPhaseChange != nil {
					go OnPhaseChange(e.Server, "halftime")
				}
			}
		}

	case "cs2.match.status":
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
		if rs.Map != "" {
			s.Map = rs.Map
		}
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
		to := e.Fields["to"]
		if p := m.getOrCreatePlayer(e.Fields["player"], e.Fields["player_steamid"], to); p != nil {
			p.Team = to
			log.Printf("[match] %s player %s switched to %s", e.Server, e.Fields["player"], to)
		}
		// Fire OnFirstPlayerJoin on the first human player's Unassigned→team switch.
		// BOTs have steamid "BOT"; humans have a Steam3 ID like "[U:1:...]".
		if !m.firstPlayerDone && e.Fields["from"] == "Unassigned" &&
			strings.HasPrefix(e.Fields["player_steamid"], "[U:") &&
			(to == "CT" || to == "TERRORIST") && OnFirstPlayerJoin != nil {
			m.firstPlayerDone = true
			steamid := steam3ToSteam64(e.Fields["player_steamid"])
			go OnFirstPlayerJoin(e.Server, steamid, to)
		}

	case "cs2.player.disconnect":
		steamid := steam3ToSteam64(e.Fields["player_steamid"])
		delete(m.readySet, steamid)
		delete(s.Players, steamid)
		log.Printf("[match] %s player left: %s", e.Server, e.Fields["player"])

	case "cs2.chat", "cs2.chat.team":
		msg := strings.TrimSpace(e.Fields["message"])
		steamid := steam3ToSteam64(e.Fields["player_steamid"])
		team := e.Fields["player_team"]
		switch msg {
		case "!ready":
			if s.Phase != PhaseWarmup || steamid == "" {
				break
			}
			m.readySet[steamid] = true
			total := m.playersToReady()
			log.Printf("[match] %s !ready %s (%d/%d)", e.Server, e.Fields["player"], len(m.readySet), total)
			if m.isAllReady(e.Server) {
				m.readySet = map[string]bool{}
				var sidePick string
				if GetEncounterInfo != nil {
					sidePick, _, _, _ = GetEncounterInfo(e.Server)
				}
				if sidePick == "knife" {
					s.Phase = PhaseKnife
					m.inKnifeSetup = true
					log.Printf("[match] %s all ready → knife", e.Server)
					if OnPhaseChange != nil {
						go OnPhaseChange(e.Server, "knife")
					}
				} else {
					s.Phase = PhaseLive
					log.Printf("[match] %s all ready → live (side_pick=%q)", e.Server, sidePick)
					if OnPhaseChange != nil {
						go OnPhaseChange(e.Server, "first_half")
					}
				}
			}
		case "!ct", "!t":
			if s.Phase != PhaseKnife || !m.knifeOver {
				break
			}
			if team != m.knifeWinSide {
				log.Printf("[match] %s !%s ignored — winner=%s player_team=%s",
					e.Server, msg[1:], m.knifeWinSide, team)
				break
			}
			winSide := m.knifeWinSide
			chosenSide := msg[1:] // "ct" or "t"
			m.knifeOver = false
			s.Phase = PhaseLive
			log.Printf("[match] %s knife winner (%s) chose %s → live", e.Server, winSide, chosenSide)
			if OnKnifeChoice != nil {
				go OnKnifeChoice(e.Server, winSide, chosenSide)
			}
		}

	case "cs2.game.over":
		if s.Phase == PhaseKnife {
			// Knife round ended — record which side won, wait for !ct/!t.
			m.knifeOver = true
			if s.ScoreCT >= s.ScoreT {
				m.knifeWinSide = "CT"
			} else {
				m.knifeWinSide = "TERRORIST"
			}
			log.Printf("[match] %s knife over CT=%d T=%d — winner=%s, waiting for !ct/!t",
				e.Server, s.ScoreCT, s.ScoreT, m.knifeWinSide)
			return // do NOT call OnGameOver
		}
		s.Phase = PhaseGameOver
		log.Printf("[match] %s → game over  CT=%d T=%d", e.Server, s.ScoreCT, s.ScoreT)
		if OnPhaseChange != nil {
			go OnPhaseChange(e.Server, "game_over")
		}
		if OnGameOver != nil {
			scoreCT, scoreT := s.ScoreCT, s.ScoreT
			go OnGameOver(e.Server, scoreCT, scoreT)
		}
	}
}

// playersToReady returns how many non-spectator players are currently known.
func (m *Machine) playersToReady() int {
	count := 0
	for _, p := range m.state.Players {
		if p.Team == "CT" || p.Team == "TERRORIST" {
			count++
		}
	}
	if count == 0 {
		return len(m.state.Players)
	}
	return count
}

// isAllReady returns true when enough players have said !ready.
func (m *Machine) isAllReady(serverAddr string) bool {
	if GetEncounterInfo != nil {
		_, readyCount, _, ok := GetEncounterInfo(serverAddr)
		if ok && readyCount > 0 {
			return len(m.readySet) >= readyCount
		}
	}
	total := m.playersToReady()
	return total > 0 && len(m.readySet) >= total
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
func (m *Machine) findByAccountID(accountID string) *PlayerStat {
	id, err := strconv.ParseInt(strings.TrimSpace(accountID), 10, 64)
	if err != nil || id == 0 {
		return nil
	}
	steam64 := strconv.FormatInt(76561197960265728+id, 10)
	return m.state.Players[steam64]
}

// steam3ToSteam64 converts "[U:1:160633]" → "76561197960426361".
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

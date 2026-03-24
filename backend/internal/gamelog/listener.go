package gamelog

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- SSE broker ---

type subscriber chan *Event

type broker struct {
	mu   sync.RWMutex
	subs map[string][]subscriber // serverAddr → subscribers
}

var Broker = &broker{subs: map[string][]subscriber{}}

func (b *broker) Subscribe(serverAddr string) subscriber {
	ch := make(subscriber, 64)
	b.mu.Lock()
	b.subs[serverAddr] = append(b.subs[serverAddr], ch)
	b.mu.Unlock()
	return ch
}

func (b *broker) Unsubscribe(serverAddr string, ch subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.subs[serverAddr]
	for i, s := range list {
		if s == ch {
			b.subs[serverAddr] = append(list[:i], list[i+1:]...)
			break
		}
	}
	close(ch)
}

func (b *broker) publish(e *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[e.Server] {
		select {
		case ch <- e:
		default:
		}
	}
}

// --- HTTP log receiver ---

// OnEvent is called for every parsed event after broker publish.
// Set this at startup to hook the match state machine.
var OnEvent func(*Event)

// HTTPHandler receives log lines POSTed by CS2 via logaddress_add_http.
// The CS2 server is identified by its remote IP.
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Identify the CS2 server by source IP
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	serverAddr := ip + ":27015"

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var (
		inJSON  bool
		jsonBuf strings.Builder
		jsonAt  time.Time
	)

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Extract timestamp + body from the log line
		m := reLogLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts, _ := time.Parse("15:04:05", m[1])
		now := time.Now()
		at := time.Date(now.Year(), now.Month(), now.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, time.Local)
		content := m[2]

		// JSON block handling
		if strings.HasPrefix(content, "JSON_BEGIN{") {
			inJSON = true
			jsonAt = at
			jsonBuf.Reset()
			jsonBuf.WriteString("{")
			continue
		}
		if inJSON {
			if strings.Contains(content, "JSON_END") {
				inJSON = false
				closing := strings.TrimSuffix(content, "JSON_END")
				jsonBuf.WriteString(closing)
				if e := parseJSONBlock(serverAddr, jsonAt, jsonBuf.String()); e != nil {
					log.Printf("[gamelog] %s [%s] round=%s CT=%s T=%s players=%d",
						serverAddr, e.Type,
						e.Fields["round"], e.Fields["score_ct"], e.Fields["score_t"],
						len(e.Extra.(*RoundStats).Players),
					)
					Broker.publish(e)
					if OnEvent != nil {
						OnEvent(e)
					}
				}
			} else {
				jsonBuf.WriteString(content + "\n")
			}
			continue
		}

		e := parse(serverAddr, line)
		if e == nil {
			log.Printf("[gamelog] unmatched: %q", content)
			continue
		}
		switch e.Type {
		case "cs2.kill", "cs2.kill.headshot":
			log.Printf("[gamelog] %s KILL  %s(%s) → %s(%s) [%s]%s",
				serverAddr,
				e.Fields["killer"], e.Fields["killer_team"],
				e.Fields["victim"], e.Fields["victim_team"],
				e.Fields["weapon"],
				map[bool]string{true: " HS", false: ""}[e.Type == "cs2.kill.headshot"],
			)
		case "cs2.score":
			log.Printf("[gamelog] %s SCORE %s: %s pts (%s joueurs)",
				serverAddr, e.Fields["team"], e.Fields["score"], e.Fields["players"])
		case "cs2.round.start":
			log.Printf("[gamelog] %s ROUND START", serverAddr)
		case "cs2.round.end":
			log.Printf("[gamelog] %s ROUND END", serverAddr)
		case "cs2.bomb.plant":
			log.Printf("[gamelog] %s BOMB  planted by %s", serverAddr, e.Fields["planter"])
		case "cs2.bomb.defuse":
			log.Printf("[gamelog] %s BOMB  defused by %s", serverAddr, e.Fields["defuser"])
		case "cs2.bomb.explode":
			log.Printf("[gamelog] %s BOMB  exploded", serverAddr)
		case "cs2.chat", "cs2.chat.team":
			log.Printf("[gamelog] %s CHAT  <%s> %s", serverAddr, e.Fields["player"], e.Fields["message"])
		case "cs2.player.connect":
			log.Printf("[gamelog] %s JOIN  %s", serverAddr, e.Fields["player"])
		case "cs2.player.disconnect":
			log.Printf("[gamelog] %s QUIT  %s", serverAddr, e.Fields["player"])
		default:
			log.Printf("[gamelog] %s [%s] %v", serverAddr, e.Type, e.Fields)
		}

		Broker.publish(e)
		if OnEvent != nil {
			OnEvent(e)
		}
	}

	w.WriteHeader(http.StatusOK)
}

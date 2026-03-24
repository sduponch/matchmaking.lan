package gamelog

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"
)

type rawRoundStats struct {
	Name        string            `json:"name"`
	RoundNumber string            `json:"round_number"`
	ScoreT      string            `json:"score_t"`
	ScoreCT     string            `json:"score_ct"`
	Map         string            `json:"map"`
	Fields      string            `json:"fields"`
	Players     map[string]string `json:"players"`
}

func parseJSONBlock(serverAddr string, at time.Time, jsonStr string) *Event {
	// CS2 omits commas between player entries (each on its own log line).
	// Fix by inserting commas where a closing quote is directly followed by
	// a newline and another opening quote: "...\n"  →  "...",\n"
	jsonStr = strings.ReplaceAll(jsonStr, "\"\n\"", "\",\n\"")

	var raw rawRoundStats
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		log.Printf("[gamelog] %s JSON parse error: %v\nJSON: %s", serverAddr, err, jsonStr)
		return nil
	}
	if raw.Name != "round_stats" {
		return nil
	}

	// Parse CSV field names from the header
	fieldNames := splitCSV(raw.Fields)

	// Parse each player
	players := map[string]*RoundPlayer{}
	for key, csvRow := range raw.Players {
		values := splitCSV(csvRow)
		p := &RoundPlayer{}
		for i, name := range fieldNames {
			if i >= len(values) {
				break
			}
			v := strings.TrimSpace(values[i])
			switch name {
			case "accountid":
				p.AccountID = v
			case "team":
				p.Team = parseInt(v)
			case "money":
				p.Money = parseInt(v)
			case "kills":
				p.Kills = parseInt(v)
			case "deaths":
				p.Deaths = parseInt(v)
			case "assists":
				p.Assists = parseInt(v)
			case "dmg":
				p.Damage = parseIntOrFloat(v)
			case "hsp":
				p.HSP = v
			case "adr":
				p.ADR = v
			case "mvp":
				p.MVP = parseInt(v)
			}
		}
		players[key] = p
	}

	stats := &RoundStats{
		Round:   parseInt(raw.RoundNumber),
		ScoreCT: parseInt(raw.ScoreCT),
		ScoreT:  parseInt(raw.ScoreT),
		Map:     raw.Map,
		Players: players,
	}

	return &Event{
		Type:   "cs2.round.stats",
		Server: serverAddr,
		At:     at,
		Fields: map[string]string{
			"round":    raw.RoundNumber,
			"score_ct": raw.ScoreCT,
			"score_t":  raw.ScoreT,
			"map":      raw.Map,
		},
		Extra: stats,
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// parseIntOrFloat handles values like "100.00" that CS2 sends as floats.
func parseIntOrFloat(s string) int {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return 0
}

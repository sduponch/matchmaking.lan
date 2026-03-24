package gamelog

import (
	"regexp"
	"time"
)

// CS2 log format: "MM/DD/YYYY - HH:MM:SS.mmm - <body>"
var reLogLine = regexp.MustCompile(`^\d{2}/\d{2}/\d{4} - (\d{2}:\d{2}:\d{2})\.\d+ - (.+)$`)

func parse(serverAddr, line string) *Event {
	m := reLogLine.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts, _ := time.Parse("15:04:05", m[1])
	now := time.Now()
	at := time.Date(now.Year(), now.Month(), now.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, time.Local)
	body := m[2]

	for _, def := range Registry {
		groups := def.re.FindStringSubmatch(body)
		if groups == nil {
			continue
		}

		fields := map[string]string{}
		for i, name := range def.re.SubexpNames() {
			if i != 0 && name != "" && groups[i] != "" {
				fields[name] = groups[i]
			}
		}

		return &Event{
			Type:   def.Type,
			Server: serverAddr,
			At:     at,
			Fields: fields,
		}
	}

	return nil
}

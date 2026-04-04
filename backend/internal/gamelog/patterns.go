package gamelog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed patterns.json
var patternsJSON []byte

// tokenTypes maps a type name to its regex template.
// {N} is replaced by the capture name at compile time.
var tokenTypes = map[string]string{
	// Source engine player: "Name<uid><steamid><team>"
	"player": `"(?P<{N}>[^<"]+)<\d+><(?P<{N}_steamid>[^>]*)><(?P<{N}_team>[^>]*)>"`,
	// Anything between double quotes
	"quoted": `"(?P<{N}>[^"]*)"`,
	// Integer (positive or negative)
	"int": `(?P<{N}>-?\d+)`,
	// Single non-space token
	"word": `(?P<{N}>\S+)`,
	// CS2 world position: [-775 -2372 164] — captured but not named
	"pos": `\[-?\d+ -?\d+ -?\d+\]`,
	// Player without team field: "Name<uid><steamid>"
	"player_nt": `"(?P<{N}>[^<"]+)<(?P<{N}_uid>\d+)><(?P<{N}_steamid>[^>]*)>"`,
}

// PatternDef associates a dot-notation event type with a DSL pattern.
type PatternDef struct {
	Type    string
	Pattern string
	re      *regexp.Regexp
}

// tokenRe matches {name} or {name:type} placeholders in a pattern string.
var tokenRe = regexp.MustCompile(`\{(\w+)(?::(\w+))?\}`)

func (p *PatternDef) compile() error {
	var sb strings.Builder
	last := 0

	for _, m := range tokenRe.FindAllStringSubmatchIndex(p.Pattern, -1) {
		// Escape the literal part before this token
		sb.WriteString(regexp.QuoteMeta(p.Pattern[last:m[0]]))

		name := p.Pattern[m[2]:m[3]]
		typ := "word"
		if m[4] != -1 {
			typ = p.Pattern[m[4]:m[5]]
		}

		tmpl, ok := tokenTypes[typ]
		if !ok {
			return fmt.Errorf("unknown token type %q in pattern %q", typ, p.Type)
		}
		sb.WriteString(strings.ReplaceAll(tmpl, "{N}", name))
		last = m[1]
	}

	sb.WriteString(regexp.QuoteMeta(p.Pattern[last:]))

	re, err := regexp.Compile(`^` + sb.String())
	if err != nil {
		return fmt.Errorf("pattern %q: %w", p.Type, err)
	}
	p.re = re
	return nil
}

// Registry holds all compiled patterns, checked in order.
var Registry []*PatternDef

func init() {
	var defs []PatternDef
	if err := json.Unmarshal(patternsJSON, &defs); err != nil {
		panic("gamelog: failed to parse patterns.json: " + err.Error())
	}

	for i := range defs {
		d := &defs[i]
		if err := d.compile(); err != nil {
			panic(err)
		}
		Registry = append(Registry, d)
	}
}

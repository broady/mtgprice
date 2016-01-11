package mtgprice

import (
	"fmt"
	"os"
	"strings"
)

type Query struct {
	Name, Rule, Type []string
	ExactName        string

	// Color may be "w", "u", "b", "r", "g", "m" (multicolored),
	// or any of the previous characters with a "!" prefix (not).
	Color []string
}

func (q *Query) Match(c *CardInfo) bool {
	if n := q.ExactName; n != "" {
		return strings.ToLower(c.Name) == n
	}
	for _, qn := range q.Name {
		if strings.Contains(strings.ToLower(c.Name), qn) {
			continue
		}
		debugf("name %q", qn)
		return false
	}
	for _, qr := range q.Rule {
		if !strings.Contains(strings.ToLower(c.Text), qr) {
			debugf("rule %q", qr)
			return false
		}
	}
	for _, qt := range q.Type {
		if strings.Contains(strings.ToLower(c.Type), qt) {
			continue
		}
		debugf("type %q", qt)
		return false
	}
Color:
	for _, qc := range q.Color {
		if len(qc) == 0 {
			continue
		}
		not := false
		if qc[0] == '!' {
			not = true
			qc = qc[1:]
		}
		if qc == "m" {
			if len(c.Colors) > 1 {
				if not {
					debugf("color !%q", qc)
					return false
				}
				continue
			}
		}
		for _, c := range c.Colors {
			if shortColor(c) == qc {
				if not {
					debugf("color !%q", qc)
					return false
				}
				continue Color
			}
		}
		if not {
			continue
		}
		debugf("color %q", qc)
		return false
	}

	return true
}

func shortColor(long string) string {
	switch long {
	case "White":
		return "w"
	case "Blue":
		return "u"
	case "Black":
		return "b"
	case "Red":
		return "r"
	case "Green":
		return "g"
	}
	return ""
}

func ParseQuery(s string) *Query {
	var q Query
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "!") {
		q.ExactName = s[1:]
		return &q
	}
	for _, s := range strings.Fields(s) {
		p := func(p string) bool { return strings.HasPrefix(s, p) }
		switch {
		case p("o:"):
			q.Rule = append(q.Rule, s[2:])
		case p("t:"):
			q.Type = append(q.Type, s[2:])
		case p("c:"):
			for _, c := range s[2:] {
				if !validColor(c) {
					continue
				}
				q.Color = append(q.Color, string(c))
			}
		case p("c!"):
			for _, c := range s[2:] {
				if !validColor(c) {
					continue
				}
				q.Color = append(q.Color, "!"+string(c))
			}
		default:
			q.Name = append(q.Name, s)
		}
	}
	return &q
}

func validColor(c rune) bool {
	switch c {
	case 'w', 'u', 'b', 'r', 'g', 'm':
		return true
	}
	return false
}

func (c *Client) Query(q string) ([]*CardInfo, error) {
	var match []*CardInfo
	query := ParseQuery(q)
	seen := map[string]bool{}
	for _, card := range c.cards {
		if query.Match(card) && !seen[card.Name] {
			match = append(match, card)
			seen[card.Name] = true
		}
	}
	return match, nil
}

const debug = false

func debugf(format string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

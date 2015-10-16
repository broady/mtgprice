package mtgprice

import (
	"fmt"
	"os"
	"strings"
)

type Query struct {
	Name, Rule, Type []string

	// Color may be "w", "u", "b", "r", "g", "m" (multicolored),
	// or any of the previous characters with a "!" prefix (not).
	Color []string
}

func (q *Query) Match(c *CardInfo) bool {
Names:
	for _, qn := range q.Name {
		if strings.Contains(strings.ToLower(c.Name), qn) {
			continue
		}
		for _, n := range c.Names {
			if strings.Contains(strings.ToLower(n), qn) {
				continue Names
			}
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
Type:
	for _, qt := range q.Type {
		if strings.Contains(strings.ToLower(c.Type), qt) {
			continue
		}
		for _, t := range c.Types {
			if strings.Contains(strings.ToLower(t), qt) {
				continue Type
			}
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
	for _, s := range strings.Fields(s) {
		p := func(p string) bool { return strings.HasPrefix(s, p) }
		switch {
		case p("o:"):
			q.Rule = append(q.Rule, strings.ToLower(s[2:]))
		case p("t:"):
			q.Type = append(q.Type, strings.ToLower(s[2:]))
		case p("c:"):
			for _, c := range strings.ToLower(s[2:]) {
				if !validColor(c) {
					continue
				}
				q.Color = append(q.Color, string(c))
			}
		case p("c!"):
			for _, c := range strings.ToLower(s[2:]) {
				if !validColor(c) {
					continue
				}
				q.Color = append(q.Color, "!"+string(c))
			}
		default:
			q.Name = append(q.Name, strings.ToLower(s))
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
	for _, card := range c.cards {
		if query.Match(card) {
			match = append(match, card)
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

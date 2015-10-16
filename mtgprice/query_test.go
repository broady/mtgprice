package mtgprice

import (
	"reflect"
	"testing"
)

func TestParseQuery(t *testing.T) {
	for _, c := range []struct {
		s string
		q Query
	}{
		{
			"blah boo bong",
			Query{Name: []string{"blah", "boo", "bong"}},
		},
		{
			"c:bw o:foo blah c!gu t:pixie",
			Query{
				Name:  []string{"blah"},
				Color: []string{"b", "w", "!g", "!u"},
				Rule:  []string{"foo"},
				Type:  []string{"pixie"},
			},
		},
	} {
		got, want := ParseQuery(c.s), &c.q
		if !reflect.DeepEqual(got, want) {
			t.Errorf("query %q produced %v, want %v", c.s, got, want)
		}
	}
}

var testCards = []*CardInfo{
	{
		Colors: []string{"Blue"},
		Name:   "True-Name Nemesis",
		Text:   "As True-Name Nemesis enters the battlefield, choose a player.\nTrue-Name Nemesis has protection from the chosen player. (This creature can't be blocked, targeted, dealt damage, or enchanted by anything controlled by that player.)",
		Type:   "Creature — Merfolk Rogue",
	},
	{
		Colors: []string{"Green"},
		Name:   "Glistener Elf",
		Text:   "Infect (This creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters.)",
		Type:   "Creature — Elf Warrior",
	},
}

func TestQueryMatch(t *testing.T) {
	q := ParseQuery("true o:battlefield c:u t:merfolk c!g")
	if !q.Match(testCards[0]) {
		t.Errorf("no match; want match")
	}
	if q.Match(testCards[1]) {
		t.Errorf("match; want no match")
	}
}

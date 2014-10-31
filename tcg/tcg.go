package tcg

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Price struct {
	Low, Mid, High int // Expressed in cents of a US dollar.
}

func Get(name string) (prices *Price, err error) {
	resp, err := http.Get("http://magic.tcgplayer.com/db/magic_single_card.asp?cn=" + url.QueryEscape(name))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got bad status code: %d", resp.StatusCode)
	}
	d, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, err
	}
	highText := d.Find("td:contains('H:')").FilterFunction(func(idx int, s *goquery.Selection) bool {
		return "H:" == strings.TrimSpace(s.Text())
	})
	if highText.Length() != 1 {
		fmt.Print(d.Html())
		return nil, fmt.Errorf("could not find prices on page (couldnt find high price tag)")
	}
	nodeText := highText.
		Parent().
		Children().
		Map(func(idx int, sel *goquery.Selection) string {
		return sel.Text()
	})
	if len(nodeText) != 7 {
		return nil, fmt.Errorf("could not find prices on page (unexpected element length for price section)")
	}
	prices = &Price{}
	prices.High, err = parsePrice(nodeText[2])
	if err != nil {
		return nil, err
	}
	prices.Mid, err = parsePrice(nodeText[4])
	if err != nil {
		return nil, err
	}
	prices.Low, err = parsePrice(nodeText[6])
	if err != nil {
		return nil, err
	}
	return prices, nil
}

// parsePrice parses a string in format "$1.00" to 100
func parsePrice(p string) (int, error) {
	p = strings.TrimSpace(p)
	if p == "" || p[0] != '$' {
		return 0, fmt.Errorf("invalid price: %s", p)
	}
	if p[len(p)-3] != '.' {
		return 0, errors.New("invalid price format. expected cents at the end")
	}
	cents := p[1:len(p)-3] + p[len(p)-2:]
	return strconv.Atoi(cents)
}

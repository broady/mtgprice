package mtgprice

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/broady/mtgprice/gatherer"
	"github.com/cznic/kv"
)

type Opts struct {
	Filename, CardData string
}

type Client struct {
	db *kv.DB
	mu sync.RWMutex

	// Immutable.
	cards map[string]*CardInfo
}

type CardInfo struct {
	Name  string   `json:"name"`
	Names []string `json:"names,omitempty"`
	// NOTE: there is one card with .5 mana cost.
	CMC          float64  `json:"cmc"`
	MultiverseID *int     `json:"multiverseid,omitempty"`
	ManaCost     string   `json:"manaCost"`
	Rarity       string   `json:"rarity"`
	Power        string   `json:"power,omitempty"`
	Toughness    string   `json:"toughness,omitempty"`
	Type         string   `json:"type"`
	Types        []string `json:"types"`
}

type Price struct {
	Low, Mid, High int // Expressed in cents of a US dollar.
}

// entry represents an entry for the database.
type entry struct {
	TCGPrice     *Price
	GathererInfo *gatherer.CardInfo
}

func Open(opts Opts) (c *Client, err error) {
	c = new(Client)
	c.cards, err = readCardData(opts.CardData)
	if err != nil {
		return
	}
	c.db, err = kv.Open(opts.Filename, &kv.Options{})
	if err != nil {
		log.Printf("Creating new database... %v", err)
		c.db, err = kv.Create(opts.Filename, &kv.Options{})
	}
	return
}

type cardInfoData struct {
	Cards []*CardInfo `json:"cards"`
}

func readCardData(fn string) (map[string]*CardInfo, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	var cardData map[string]cardInfoData
	if err := json.NewDecoder(f).Decode(&cardData); err != nil {
		return nil, fmt.Errorf("could not read card json file: %v", err)
	}
	out := make(map[string]*CardInfo)
	for _, set := range cardData {
		for _, card := range set.Cards {
			if len(card.Names) != 0 {
				out[strings.ToLower(strings.Join(card.Names, " & "))] = card
				out[strings.ToLower(strings.Join(card.Names, " / "))] = card
				out[strings.ToLower(strings.Join(card.Names, " // "))] = card
			}
			card.Name = normalizeCardName(card.Name)
			out[strings.ToLower(card.Name)] = card
		}
	}
	return out, nil
}

func (c *Client) Close() error {
	if c != nil && c.db != nil {
		return c.db.Close()
	}
	return nil
}

func parsePrices(in [3]string) (out Price, err error) {
	out.Low, err = parsePrice(in[0])
	if err != nil {
		return
	}
	out.Mid, err = parsePrice(in[1])
	if err != nil {
		return
	}
	out.High, err = parsePrice(in[2])
	return
}

// parsePrice parses a string in format "$1.00" to 100
func parsePrice(p string) (int, error) {
	if p == "" || p[0] != '$' {
		return 0, fmt.Errorf("invalid price: %s", p)
	}
	if p[len(p)-3] != '.' {
		return 0, errors.New("invalid price format. expected cents at the end")
	}
	cents := p[1:len(p)-3] + p[len(p)-2:]
	return strconv.Atoi(cents)
}

func normalizeCardName(s string) string {
	s = strings.Replace(s, "Æ", "Ae", -1)
	return strings.Replace(s, "’", "'", -1)
}

func (c *Client) CardInfo(cardName string) (ci CardInfo, ok bool) {
	card, ok := c.cards[strings.ToLower(normalizeCardName(cardName))]
	if ok {
		return *card, ok
	}
	return
}

func (c *Client) getEntry(cardName string) (result entry, err error) {
	cardName = normalizeCardName(cardName)
	e := new(entry)
	ci, ok := c.CardInfo(cardName)
	if !ok {
		return *e, errors.New("card not found")
	}
	err = c.get(ci.Name, e)
	if err != nil && err != doesNotExistError {
		return *e, err
	}
	if err == nil && e.TCGPrice != nil && e.GathererInfo != nil {
		//log.Printf("cache hit: %s", cardName)
		return *e, nil
	}
	if e.TCGPrice == nil {
		log.Printf("fetching tcg price: %s", ci.Name)
		prices, err := c.priceForCard(ci)
		if err != nil {
			return *e, err
		}
		e.TCGPrice = &prices
		go c.set(ci.Name, e)
	}
	if e.GathererInfo == nil {
		name := ci.Name
		if len(ci.Names) != 0 {
			name = strings.Join(ci.Names, " & ")
		}
		log.Printf("fetching gatherer data: %s", name)
		var gInfo *gatherer.CardInfo
		//if ci.MultiverseID != nil {
		//gInfo, err = gatherer.Info(*ci.MultiverseID)
		//} else {
		gInfo, err = gatherer.InfoByName(name)
		//}
		if err != nil {
			return *e, err
		}
		e.GathererInfo = gInfo
		go c.set(ci.Name, e)
	}
	return *e, nil
}

type info struct {
	entry
	CardInfo
}

func (c *Client) RichInfo(cardName string) (i info, err error) {
	ci, ok := c.CardInfo(cardName)
	if !ok {
		return i, errors.New("could not find card")
	}
	i.CardInfo = ci

	e, err := c.getEntry(cardName)
	if err != nil {
		return
	}
	i.entry = e
	return
}

func (c *Client) PriceForCard(cardName string) (prices Price, err error) {
	e, err := c.getEntry(cardName)
	if err != nil {
		return
	}
	prices = *e.TCGPrice
	return
}

var doesNotExistError = errors.New("val does not exist")

func (c *Client) get(key string, val interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, err := c.db.Get(nil, []byte(key))
	if err != nil {
		return err
	}
	if len(v) == 0 {
		return doesNotExistError
	}
	return gob.NewDecoder(bytes.NewBuffer(v)).Decode(val)
}

func (c *Client) set(key string, val interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(val)
	if err != nil {
		return err
	}
	err = c.db.Set([]byte(key), b.Bytes())
	if err != nil {
		log.Printf("ERROR writing to cache: %v", err)
	}
	return err
}

func (c *Client) priceForCard(ci CardInfo) (prices Price, err error) {
	name := ci.Name
	if len(ci.Names) != 0 {
		name = strings.Join(ci.Names, " // ")
	}
	resp, err := http.Get("http://magictcgprices.appspot.com/api/tcgplayer/price.json?cardname=" +
		url.QueryEscape(name))
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var in [3]string
	err = json.Unmarshal(body, &in)
	if err != nil {
		log.Printf("error parsing prices: %v - %s", err, body)
		return
	}
	return parsePrices(in)
}

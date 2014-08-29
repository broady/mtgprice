package mtgprice

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cznic/kv"
)

type Opts struct {
	Filename, CardData string
}

type Client struct {
	db *kv.DB
	mu sync.RWMutex

	// Immutable.
	cards map[string]CardInfo
}

type CardInfo struct {
	Name string `json:"name"`
	// NOTE: there is one card with .5 mana cost.
	CMC       float64  `json:"cmc"`
	ManaCost  string   `json:"manaCost"`
	Rarity    string   `json:"rarity"`
	Power     string   `json:"power,omitempty"`
	Toughness string   `json:"toughness,omitempty"`
	Type      string   `json:"type"`
	Types     []string `json:"types"`
}

type Price struct {
	Low, Mid, High int // Expressed in cents of a US dollar.
}

// entry represents an entry for the database.
type entry struct {
	TCGPrice        *Price
	CommunityRating *float64
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
	Cards []CardInfo `json:"cards"`
}

func readCardData(fn string) (map[string]CardInfo, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	var cardData map[string]cardInfoData
	if err := json.NewDecoder(f).Decode(&cardData); err != nil {
		return nil, fmt.Errorf("could not read card json file: %v", err)
	}
	out := make(map[string]CardInfo)
	for _, set := range cardData {
		for _, card := range set.Cards {
			card.Name = strings.Replace(card.Name, "Æ", "Ae", -1)
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
		return 0, errors.New("invalid price")
	}
	if p[len(p)-3] != '.' {
		return 0, errors.New("invalid price format. expected cents at the end")
	}
	cents := p[1:len(p)-3] + p[len(p)-2:]
	return strconv.Atoi(cents)
}

func (c *Client) CardInfo(cardName string) (card CardInfo, ok bool) {
	card, ok = c.cards[strings.ToLower(cardName)]
	return
}

func (c *Client) PriceForCard(cardName string) (prices Price, err error) {
	// TODO: handle race condition properly.
	entry := new(entry)
	err = c.get(cardName, entry)
	if err == nil && entry.TCGPrice != nil {
		log.Printf("cache hit: %s", cardName)
		return *entry.TCGPrice, nil
	}
	if err == doesNotExistError || entry.TCGPrice == nil {
		log.Printf("cache miss, fetching from network: %s", cardName)
		prices, err = c.priceForCard(cardName)
		if err != nil {
			return
		}
		entry.TCGPrice = &prices
		go c.set(cardName, entry)
	}
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

func (c *Client) priceForCard(cardName string) (prices Price, err error) {
	resp, err := http.Get("http://magictcgprices.appspot.com/api/tcgplayer/price.json?cardname=" +
		url.QueryEscape(cardName))
	if err != nil {
		return
	}
	var in [3]string
	err = json.NewDecoder(resp.Body).Decode(&in)
	if err != nil {
		return
	}
	return parsePrices(in)
}

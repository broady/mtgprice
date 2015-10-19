package mtgprice

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/broady/mtgprice/gatherer"
	"github.com/broady/mtgprice/tcg"
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
	Colors       []string `json:"colors"`
	Text         string   `json:"text"`
}

func (c *CardInfo) String() string {
	return c.Name
}

func (c *CardInfo) Detail() string {
	s := fmt.Sprintf("%v %v", c.Name, c.ManaCost)
	for _, t := range c.Types {
		if t == "Creature" {
			s += fmt.Sprintf(" (%v/%v)", c.Power, c.Toughness)
		}
	}
	s += "\n" + c.Type
	if c.Text != "" {
		s += "\n" + c.Text
	}
	return s
}

// entry represents an entry for the database.
type entry struct {
	TCGPrice     *tcg.Price
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

func readCardData(fn string) (map[string]*CardInfo, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	var cardData map[string]*CardInfo
	if err := json.NewDecoder(f).Decode(&cardData); err != nil {
		return nil, fmt.Errorf("could not read card json file: %v", err)
	}
	// Normalize.
	out := make(map[string]*CardInfo)
	for _, card := range cardData {
		if len(card.Names) != 0 {
			out[strings.ToLower(strings.Join(card.Names, " & "))] = card
			out[strings.ToLower(strings.Join(card.Names, " / "))] = card
			out[strings.ToLower(strings.Join(card.Names, " // "))] = card
		}
		card.Name = normalizeCardName(card.Name)
		out[strings.ToLower(card.Name)] = card
	}
	return out, nil
}

func (c *Client) Close() error {
	if c != nil && c.db != nil {
		return c.db.Close()
	}
	return nil
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

func (c *Client) getEntry(cardName string) (e entry, err error) {
	cardName = normalizeCardName(cardName)
	ci, ok := c.CardInfo(cardName)
	if !ok {
		return e, errors.New("card not found")
	}
	err = c.get(ci.Name, &e)
	if err != nil && err != doesNotExistError {
		return e, err
	}
	if err == nil && e.TCGPrice != nil && e.GathererInfo != nil {
		//log.Printf("cache hit: %s", cardName)
		return e, nil
	}
	touch := false
	if e.TCGPrice == nil {
		prices, err := c.priceForCard(ci)
		if err != nil {
			log.Println("Fetching price:", err)
		} else {
			e.TCGPrice = prices
			touch = true
		}
	}
	if e.GathererInfo == nil {
		name := ci.Name
		if len(ci.Names) != 0 {
			name = strings.Join(ci.Names, " & ")
		}
		gInfo, err := gatherer.InfoByName(name)
		if err != nil {
			log.Println("Fetching gatherer:", err)
		} else {
			e.GathererInfo = gInfo
			touch = true
		}
	}
	if touch {
		go c.set(ci.Name, e)
	}
	return e, nil
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

func (c *Client) PriceForCard(cardName string) (prices tcg.Price, err error) {
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

func (c *Client) priceForCard(ci CardInfo) (prices *tcg.Price, err error) {
	name := ci.Name
	if len(ci.Names) != 0 {
		name = strings.Join(ci.Names, " // ")
	}
	return tcg.Get(name)
}

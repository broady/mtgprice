package gatherer

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

type CardInfo struct {
	CommunityRating float64
	CommunityVotes  int
}

var (
	ratingRegexp = regexp.MustCompile(`class="textRatingValue">([\d.]*)</span>`)
	votesRegexp  = regexp.MustCompile(`class="totalVotesValue">([\d]*)</span>`)
)

func byUrl(url string) (*CardInfo, error) {
	resp, err := http.Get(url)
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
	rating, err := strconv.ParseFloat(d.Find(".textRating .textRatingValue").First().Text(), 64)
	if err != nil {
		if html, err := d.Html(); err == nil {
			log.Print(html)
		}
		return nil, err
	}
	votes, err := strconv.Atoi(d.Find(".textRating .totalVotesValue").First().Text())
	if err != nil {
		return nil, err
	}
	return &CardInfo{
		CommunityRating: rating,
		CommunityVotes:  votes,
	}, nil
}

func Info(multiverseID int) (*CardInfo, error) {
	url := fmt.Sprintf("http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=%d", multiverseID)
	return byUrl(url)
}

func InfoByName(cardName string) (*CardInfo, error) {
	url := fmt.Sprintf("http://gatherer.wizards.com/Pages/Card/Details.aspx?name=%s",
		url.QueryEscape(cardName))
	return byUrl(url)
}

package gatherer

import (
	"errors"
	"fmt"
	"io/ioutil"
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

func getPageByName(cardName string) ([]byte, error) {
	resp, err := http.Get("http://gatherer.wizards.com/Pages/Card/Details.aspx?name=" +
		url.QueryEscape(cardName))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return ioutil.ReadAll(resp.Body)
	}
	return nil, errors.New("http response error")
}

func getPageByID(multiverseID int) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=%d", multiverseID))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return ioutil.ReadAll(resp.Body)
	}
	return nil, errors.New("http response error")
}

func byUrl(url string) (*CardInfo, error) {
	d, err := goquery.NewDocument(url)
	if err != nil {
		return nil, err
	}
	rating, err := strconv.ParseFloat(d.Find(".textRating .textRatingValue").First().Text(), 64)
	if err != nil {
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

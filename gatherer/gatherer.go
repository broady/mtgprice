package gatherer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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

func Info(multiverseID int) (i *CardInfo, err error) {
	page, err := getPageByID(multiverseID)
	if err != nil {
		return
	}
	matches := ratingRegexp.FindAllSubmatch(page, 1)
	if matches == nil {
		return nil, errors.New("could not find rating on page")
	}
	rating, err := strconv.ParseFloat(string(matches[0][1]), 64)
	if err != nil {
		return nil, err
	}

	matches = votesRegexp.FindAllSubmatch(page, 1)
	if matches == nil {
		return nil, errors.New("could not find num votes on page")
	}
	votes, err := strconv.Atoi(string(matches[0][1]))
	if err != nil {
		return nil, err
	}
	return &CardInfo{
		CommunityRating: rating,
		CommunityVotes:  votes,
	}, nil
}

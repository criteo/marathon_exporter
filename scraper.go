package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Scraper interface {
	Scrape(path string) ([]byte, error)
}

type scraper struct {
	uri *url.URL
}

func (s *scraper) Scrape(path string) ([]byte, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	//response, err := client.Get(fmt.Sprintf("%v/%s", s.uri, path))
	mURL := fmt.Sprintf("%v/%s", s.uri, path)
	reqest, err := http.NewRequest("GET", mURL, nil)
	if err != nil {
		return nil, err
	}
	reqest.SetBasicAuth(*marathonUserName, *marathonPassword)
	response, err := client.Do(reqest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, err
}

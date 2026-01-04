package client

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

type Client struct {
	HTTP *http.Client
}

func New(switchIP string) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	return &Client{
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *Client) StatsReq(switchIP, cookie, referer string) *http.Request {
	u := &url.URL{Scheme: "http", Host: switchIP, Path: "/port.cgi?page=stats"}
	req, _ := http.NewRequest("GET", u.String(), nil)
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Referer", "http://"+switchIP+"/"+referer)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	
	return req
}


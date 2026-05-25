package nyaa

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Torrent struct {
	Title    string
	Size     string
	Seeders  int
	Leechers int
	InfoHash string
	Torrent  string
}

func (t Torrent) Magnet() string {
	base := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", t.InfoHash, url.QueryEscape(t.Title))
	// Standard public trackers for fast connections
	trackers := []string{
		"http://nyaa.tracker.wf:7777/announce",
		"udp://tracker.coppersurfer.tk:6969/announce",
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://open.stealth.si:80/announce",
		"udp://tracker.internetwarriors.net:1337/announce",
	}
	for _, tr := range trackers {
		base += "&tr=" + url.QueryEscape(tr)
	}
	return base
}

type rssRoot struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	Size     string `xml:"size"`
	Seeders  string `xml:"seeders"`
	Leechers string `xml:"leechers"`
	InfoHash string `xml:"infoHash"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		baseURL: "https://nyaa.si",
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]Torrent, error) {
	escapedQuery := url.QueryEscape(query)
	// c=1_2: Anime - English-translated
	reqURL := fmt.Sprintf("%s/?page=rss&q=%s&c=1_2&f=0", c.baseURL, escapedQuery)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ani-cli/0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Nyaa RSS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from Nyaa: %d", resp.StatusCode)
	}

	var root rssRoot
	if err := xml.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("failed to decode Nyaa XML response: %w", err)
	}

	var results []Torrent
	for _, item := range root.Channel.Items {
		seeds, _ := strconv.Atoi(item.Seeders)
		leechs, _ := strconv.Atoi(item.Leechers)

		results = append(results, Torrent{
			Title:    item.Title,
			Size:     item.Size,
			Seeders:  seeds,
			Leechers: leechs,
			InfoHash: item.InfoHash,
			Torrent:  item.Link,
		})
	}

	return results, nil
}

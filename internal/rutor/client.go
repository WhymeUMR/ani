package rutor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Torrent struct {
	Title    string
	Size     string
	Seeders  int
	Leechers int
	Magnet   string
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		baseURL: "https://rutor.is",
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

var (
	rowRegexp    = regexp.MustCompile(`(?s)<tr class="(?:gai|tum)">.*?</tr>`)
	magnetRegexp = regexp.MustCompile(`href="(magnet:\?xt=urn:btih:([a-fA-F0-9]{40})[^"]*)"`)
	titleRegexp  = regexp.MustCompile(`href="/torrent/\d+/[^"]+">([^<]+)</a>`)
	sizeRegexp   = regexp.MustCompile(`<td align="right">([^<]+(?:GB|MB|KB|TB|&nbsp;GB|&nbsp;MB|&nbsp;KB|&nbsp;TB))</td>`)
	seedsRegexp  = regexp.MustCompile(`class="green">.*?&nbsp;(\d+)</span>`)
	leechsRegexp = regexp.MustCompile(`class="red">&nbsp;(\d+)</span>`)
)

func (c *Client) Search(ctx context.Context, query string) ([]Torrent, error) {
	escapedQuery := url.QueryEscape(query)
	reqURL := fmt.Sprintf("%s/search/0/0/000/0/%s", c.baseURL, escapedQuery)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ani-cli/0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Rutor search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from Rutor: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Rutor response body: %w", err)
	}

	body := string(bodyBytes)
	rows := rowRegexp.FindAllString(body, -1)

	var results []Torrent
	for _, row := range rows {
		magnetMatch := magnetRegexp.FindStringSubmatch(row)
		titleMatch := titleRegexp.FindStringSubmatch(row)
		sizeMatch := sizeRegexp.FindStringSubmatch(row)
		seedsMatch := seedsRegexp.FindStringSubmatch(row)
		leechsMatch := leechsRegexp.FindStringSubmatch(row)

		if len(magnetMatch) < 2 || len(titleMatch) < 2 {
			continue
		}

		magnet := magnetMatch[1]
		// Convert HTML entities like &amp; in magnet link
		magnet = strings.ReplaceAll(magnet, "&amp;", "&")

		title := strings.TrimSpace(titleMatch[1])

		size := "unknown"
		if len(sizeMatch) >= 2 {
			size = strings.ReplaceAll(sizeMatch[1], "&nbsp;", " ")
		}

		seeds := 0
		if len(seedsMatch) >= 2 {
			seeds, _ = strconv.Atoi(seedsMatch[1])
		}

		leechs := 0
		if len(leechsMatch) >= 2 {
			leechs, _ = strconv.Atoi(leechsMatch[1])
		}

		results = append(results, Torrent{
			Title:    title,
			Size:     size,
			Seeders:  seeds,
			Leechers: leechs,
			Magnet:   magnet,
		})
	}

	return results, nil
}

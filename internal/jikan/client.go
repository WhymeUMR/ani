package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Anime struct {
	ID       int     `json:"mal_id"`
	Title    string  `json:"title"`
	Score    float64 `json:"score"`
	Type     string  `json:"type"`
	Episodes int     `json:"episodes"`
	Status   string  `json:"status"`
	Year     int     `json:"year"`
}

type apiResponse struct {
	Data []Anime `json:"data"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "https://api.jikan.moe/v4"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) GetTopAnime(ctx context.Context) ([]Anime, error) {
	reqURL := fmt.Sprintf("%s/top/anime?limit=10", c.baseURL)
	return c.fetch(ctx, reqURL)
}

func (c *Client) SearchAnime(ctx context.Context, query string) ([]Anime, error) {
	escaped := url.QueryEscape(query)
	reqURL := fmt.Sprintf("%s/anime?q=%s&limit=10", c.baseURL, escaped)
	return c.fetch(ctx, reqURL)
}

func (c *Client) fetch(ctx context.Context, reqURL string) ([]Anime, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ani-cli/0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded (429), please try again later")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode json response: %w", err)
	}

	return result.Data, nil
}

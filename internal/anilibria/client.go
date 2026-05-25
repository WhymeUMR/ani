package anilibria

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ReleaseName struct {
	Main    string `json:"main"`
	English string `json:"english"`
}

type Release struct {
	ID          int       `json:"id"`
	Year        int       `json:"year"`
	Name        ReleaseName `json:"name"`
	Description string    `json:"description"`
	Torrents    []Torrent `json:"torrents"`
}

type TorrentQuality struct {
	Value string `json:"value"`
}

type Torrent struct {
	ID          int            `json:"id"`
	Hash        string         `json:"hash"`
	Size        int64          `json:"size"`
	Label       string         `json:"label"`
	Magnet      string         `json:"magnet"`
	Seeders     int            `json:"seeders"`
	Leechers    int            `json:"leechers"`
	Quality     TorrentQuality `json:"quality"`
	Description string         `json:"description"`
}

type catalogFilter struct {
	Search string `json:"search"`
}

type catalogRequestBody struct {
	F catalogFilter `json:"f"`
}

type searchResponse struct {
	Data []Release `json:"data"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		baseURL: "https://anilibria.top/api/v1",
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]Release, error) {
	reqURL := fmt.Sprintf("%s/anime/catalog/releases", c.baseURL)

	body := catalogRequestBody{
		F: catalogFilter{
			Search: query,
		},
	}

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ani-cli/0.0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

func (c *Client) GetRelease(ctx context.Context, id int) (*Release, error) {
	reqURL := fmt.Sprintf("%s/anime/releases/%d", c.baseURL, id)

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

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release not found (404)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &release, nil
}

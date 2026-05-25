package anilibria

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAniLibriaSearch_Success(t *testing.T) {
	mockResponse := `{
		"data": [
			{
				"id": 10085,
				"year": 2026,
				"name": {
					"main": "Провожающая в последний путь Фрирен 2",
					"english": "Sousou no Frieren 2nd Season"
				},
				"description": "Описание Frieren"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime/catalog/releases" {
			t.Errorf("expected path /anime/catalog/releases, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL // Override for testing

	releases, err := client.Search(context.Background(), "Frieren")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}

	rel := releases[0]
	if rel.ID != 10085 || rel.Name.Main != "Провожающая в последний путь Фрирен 2" {
		t.Errorf("incorrect fields in parsed release: %+v", rel)
	}
}

func TestAniLibriaGetRelease_Success(t *testing.T) {
	mockResponse := `{
		"id": 10085,
		"year": 2026,
		"name": {
			"main": "Фрирен 2",
			"english": "Frieren 2"
		},
		"torrents": [
			{
				"id": 37648,
				"hash": "21a3e4eb0f8b2fbc2a7ae055522bfdb7d7d458ca",
				"size": 3266128953,
				"label": "Frieren - AniLibria [1080p HEVC][1-10]",
				"magnet": "magnet:?xt=urn:btih:21a3e4eb0f8b2fbc2a7ae055522bfdb7d7d458ca",
				"seeders": 73,
				"leechers": 2,
				"quality": {
					"value": "1080p"
				},
				"description": "1-10"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime/releases/10085" {
			t.Errorf("expected path /anime/releases/10085, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	rel, err := client.GetRelease(context.Background(), 10085)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rel.ID != 10085 || len(rel.Torrents) != 1 {
		t.Fatalf("incorrect parsed release detail: %+v", rel)
	}

	tor := rel.Torrents[0]
	if tor.ID != 37648 || tor.Quality.Value != "1080p" || tor.Seeders != 73 {
		t.Errorf("incorrect parsed torrent fields: %+v", tor)
	}
}

func TestAniLibriaGetRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	_, err := client.GetRelease(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "release not found (404)"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestAniLibriaGetRelease_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	_, err := client.GetRelease(context.Background(), 10085)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAniLibriaGetRelease_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	_, err := client.GetRelease(context.Background(), 10085)
	if err == nil {
		t.Fatal("expected error parsing invalid JSON, got nil")
	}
}

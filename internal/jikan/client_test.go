package jikan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetTopAnime_Success(t *testing.T) {
	mockResponse := `{
		"data": [
			{"mal_id": 1, "title": "Steins;Gate", "score": 9.07, "type": "TV", "episodes": 24, "status": "Finished Airing", "year": 2011},
			{"mal_id": 2, "title": "Chainsaw Man", "score": 8.5, "type": "TV", "episodes": 12, "status": "Finished Airing", "year": 2022}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/top/anime" {
			t.Errorf("expected path /top/anime, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit 10, got %s", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	list, err := client.GetTopAnime(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}

	if list[0].Title != "Steins;Gate" || list[0].Score != 9.07 {
		t.Errorf("incorrect fields for first item: %+v", list[0])
	}
}

func TestSearchAnime_Success(t *testing.T) {
	mockResponse := `{
		"data": [
			{"mal_id": 3, "title": "Naruto", "score": 7.9, "type": "TV", "episodes": 220, "status": "Finished Airing", "year": 2002}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime" {
			t.Errorf("expected path /anime, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "Naruto" {
			t.Errorf("expected query Naruto, got %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	list, err := client.SearchAnime(context.Background(), "Naruto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 item, got %d", len(list))
	}

	if list[0].Title != "Naruto" || list[0].ID != 3 {
		t.Errorf("incorrect fields for item: %+v", list[0])
	}
}

func TestClient_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	_, err := client.GetTopAnime(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "rate limit exceeded (429), please try again later"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestClient_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	_, err := client.GetTopAnime(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, err) { // simple check to ensure we get some error
		t.Errorf("expected error, got %v", err)
	}
}

func TestClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	_, err := client.GetTopAnime(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_TimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set tiny timeout to trigger it
	client := NewClient(server.URL, 1*time.Millisecond)
	_, err := client.GetTopAnime(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 2*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.GetTopAnime(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

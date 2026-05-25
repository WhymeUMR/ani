package nyaa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNyaaSearch_Success(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
	<channel>
		<title>Nyaa - RSS</title>
		<item>
			<title>[JacobSwaggedUp] Steins;Gate (BD 1280x720) [MP4 Batch]</title>
			<link>https://nyaa.si/download/2052218.torrent</link>
			<nyaa:seeders>15</nyaa:seeders>
			<nyaa:leechers>3</nyaa:leechers>
			<nyaa:infoHash>fdad33e918144de60793cb68aaa4569423bbf9c5</nyaa:infoHash>
			<nyaa:size>5.4 GiB</nyaa:size>
		</item>
	</channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "Steins Gate" {
			t.Errorf("expected query 'Steins Gate', got %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockXML))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL // Override base URL for testing

	list, err := client.Search(context.Background(), "Steins Gate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}

	tor := list[0]
	if tor.Title != "[JacobSwaggedUp] Steins;Gate (BD 1280x720) [MP4 Batch]" {
		t.Errorf("unexpected title: %s", tor.Title)
	}
	if tor.Seeders != 15 || tor.Leechers != 3 {
		t.Errorf("unexpected seeders/leechers: %d/%d", tor.Seeders, tor.Leechers)
	}
	if tor.Size != "5.4 GiB" {
		t.Errorf("unexpected size: %s", tor.Size)
	}
	if tor.InfoHash != "fdad33e918144de60793cb68aaa4569423bbf9c5" {
		t.Errorf("unexpected infohash: %s", tor.InfoHash)
	}

	// Test Magnet link generation
	magnet := tor.Magnet()
	if !strings.HasPrefix(magnet, "magnet:?xt=urn:btih:fdad33e918144de60793cb68aaa4569423bbf9c5") {
		t.Errorf("magnet link doesn't contain infohash: %s", magnet)
	}
	if !strings.Contains(magnet, "dn=%5BJacobSwaggedUp%5D+Steins%3BGate") {
		t.Errorf("magnet link doesn't contain escaped title: %s", magnet)
	}
	if !strings.Contains(magnet, "tr=http%3A%2F%2Fnyaa.tracker.wf%3A7777%2Fannounce") {
		t.Errorf("magnet link doesn't contain trackers: %s", magnet)
	}
}

func TestNyaaSearch_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<invalid xml`))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error parsing invalid XML, got nil")
	}
}

func TestNyaaSearch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error from 500 status code, got nil")
	}
}

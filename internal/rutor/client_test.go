package rutor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRutorSearch_Success(t *testing.T) {
	mockHTML := `<html>
<body>
<div id="index">
<table>
<tr class="gai">
	<td>05&nbsp;Апр&nbsp;26</td>
	<td><a class="downgif" href="//d.rutor.info/download/1070124"><img src="//cdnbunny.org/i/d.gif" alt="D" /></a>
	<a href="magnet:?xt=urn:btih:4be23e453cdc4c2915fc93dd5a8982dc70516cd2&amp;dn=rutor.info"><img src="//cdnbunny.org/i/m.png" alt="M" /></a>
	<a href="/torrent/1070124/provozhajuwaja-v-poslednij-put-friren">Провожающая в последний путь Фрирен [S02] | StudioBand, JAM Club</a></td>
	<td align="right">5<img src="//cdnbunny.org/i/com.gif" /></td>
	<td align="right">15.74&nbsp;GB</td>
	<td align="center"><span class="green"><img />&nbsp;67</span>&nbsp;<img /><span class="red">&nbsp;13</span></td>
</tr>
</table>
</div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/0/0/000/0/Frieren" {
			t.Errorf("expected path /search/0/0/000/0/Frieren, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL // Override for testing

	torrents, err := client.Search(context.Background(), "Frieren")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(torrents))
	}

	tor := torrents[0]
	if tor.Title != "Провожающая в последний путь Фрирен [S02] | StudioBand, JAM Club" {
		t.Errorf("unexpected title: %s", tor.Title)
	}
	if tor.Size != "15.74 GB" {
		t.Errorf("unexpected size: %s", tor.Size)
	}
	if tor.Seeders != 67 || tor.Leechers != 13 {
		t.Errorf("unexpected seeders/leechers: %d/%d", tor.Seeders, tor.Leechers)
	}
	expectedMagnet := "magnet:?xt=urn:btih:4be23e453cdc4c2915fc93dd5a8982dc70516cd2&dn=rutor.info"
	if tor.Magnet != expectedMagnet {
		t.Errorf("unexpected magnet: %s", tor.Magnet)
	}
}

func TestRutorSearch_ServerError(t *testing.T) {
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

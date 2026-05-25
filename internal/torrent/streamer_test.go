package torrent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStreamer_InitAndClose(t *testing.T) {
	tempCacheDir := filepath.Join(".", ".test_cache_torrents")
	
	// Ensure cache directory clean before test
	os.RemoveAll(tempCacheDir)

	streamer, err := NewStreamer(tempCacheDir)
	if err != nil {
		t.Fatalf("failed to create NewStreamer: %v", err)
	}

	if streamer.client == nil {
		t.Error("expected client to be initialized, got nil")
	}

	// Verify cache dir created
	if _, err := os.Stat(tempCacheDir); os.IsNotExist(err) {
		t.Errorf("expected cache directory %q to be created", tempCacheDir)
	}

	// Close streamer and verify cleanup
	streamer.Close()

	if _, err := os.Stat(tempCacheDir); !os.IsNotExist(err) {
		t.Errorf("expected cache directory %q to be deleted after Close()", tempCacheDir)
	}
}

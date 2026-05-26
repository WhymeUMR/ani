package torrent

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

type TorrentFile struct {
	Index int
	Name  string
	Size  int64
}

type Streamer struct {
	client   *torrent.Client
	tor      *torrent.Torrent
	cacheDir string
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
}

func NewStreamer(cacheDir string) (*Streamer, error) {
	if cacheDir == "" {
		// Use local project directory for cache by default
		cacheDir = filepath.Join(".", ".cache", "torrents")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = cacheDir
	cfg.Seed = false // Disable seeding by default to save traffic

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to start torrent client: %w", err)
	}

	return &Streamer{
		client:   client,
		cacheDir: cacheDir,
	}, nil
}

func (s *Streamer) LoadMagnet(magnet string) ([]TorrentFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If there's an existing torrent loaded, close it
	if s.tor != nil {
		s.tor.Drop()
	}

	tor, err := s.client.AddMagnet(magnet)
	if err != nil {
		return nil, fmt.Errorf("failed to add magnet link: %w", err)
	}

	// Wait for torrent metadata with a timeout to prevent hanging on dead torrents
	select {
	case <-tor.GotInfo():
		// Metadata received successfully
	case <-time.After(20 * time.Second):
		tor.Drop()
		return nil, fmt.Errorf("timeout fetching torrent metadata (no active seeders or slow connection)")
	}
	s.tor = tor

	var files []TorrentFile
	for i, file := range tor.Files() {
		files = append(files, TorrentFile{
			Index: i,
			Name:  file.DisplayPath(),
			Size:  file.Length(),
		})
	}

	return files, nil
}

func (s *Streamer) StartStreaming(fileIndex int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tor == nil {
		return "", fmt.Errorf("no torrent loaded")
	}

	files := s.tor.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return "", fmt.Errorf("invalid file index: %d", fileIndex)
	}

	targetFile := files[fileIndex]

	// Enable sequential downloading for this file
	targetFile.Download()

	// Start local HTTP server on a random free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to bind local port: %w", err)
	}
	s.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		// Create a torrent reader which prioritizes downloading sequential pieces requested by the player
		reader := targetFile.NewReader()
		defer reader.Close()

		// ServeContent handles HTTP Range requests (Seek/Rewind) natively!
		http.ServeContent(w, r, targetFile.DisplayPath(), time.Time{}, reader)
	})

	s.server = &http.Server{
		Handler: mux,
	}

	// Run HTTP server in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	streamURL := fmt.Sprintf("http://127.0.0.1:%d/stream", port)
	return streamURL, nil
}

func (s *Streamer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop HTTP server
	if s.server != nil {
		s.server.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}

	// Drop loaded torrent
	if s.tor != nil {
		s.tor.Drop()
	}

	// Close torrent client
	if s.client != nil {
		s.client.Close()
	}

	// Clean cache directory
	if err := os.RemoveAll(s.cacheDir); err != nil {
		fmt.Printf("Warning: failed to clear torrent cache: %v\n", err)
	}
}

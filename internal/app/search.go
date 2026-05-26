package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WhymeUMR/ani/internal/anilibria"
	"github.com/WhymeUMR/ani/internal/nyaa"
	"github.com/WhymeUMR/ani/internal/rutor"
)

// searchAllSources ищет торренты на всех источниках параллельно
func searchAllSources(ctx context.Context, query string, animeTitleEn string, epNum int) []PlayableTorrent {
	var mu sync.Mutex
	var results []PlayableTorrent
	var wg sync.WaitGroup

	// 1. Поиск на AniLibria
	wg.Add(1)
	go func() {
		defer wg.Done()
		alClient := anilibria.NewClient(15 * time.Second)
		queries := []string{animeTitleEn}
		if query != animeTitleEn && query != "" {
			queries = append(queries, query)
		}

		visitedReleases := make(map[int]bool)
		for _, q := range queries {
			releases, err := alClient.Search(ctx, q)
			if err != nil {
				continue
			}
			for _, r := range releases {
				if visitedReleases[r.ID] {
					continue
				}
				visitedReleases[r.ID] = true

				detail, err := alClient.GetRelease(ctx, r.ID)
				if err != nil {
					continue
				}

				for _, t := range detail.Torrents {
					if matchAniLibriaTorrent(t.Description, epNum) {
						mu.Lock()
						results = append(results, PlayableTorrent{
							Source:    "AniLibria",
							Title:     fmt.Sprintf("%s (%s)", r.Name.Main, t.Description),
							Size:      fmt.Sprintf("%.2f GiB", float64(t.Size)/(1024*1024*1024)),
							Seeders:   t.Seeders,
							Magnet:    t.Magnet,
							Voiceover: "AniLibria",
						})
						mu.Unlock()
					}
				}
			}
		}
	}()

	// 2. Поиск на Nyaa (Оригинал + Субтитры)
	wg.Add(1)
	go func() {
		defer wg.Done()
		nyaaClient := nyaa.NewClient(15 * time.Second)
		searchQueries := []string{
			fmt.Sprintf("%s %02d", animeTitleEn, epNum),
			fmt.Sprintf("%s %d", animeTitleEn, epNum),
			animeTitleEn,
		}

		visitedMagnets := make(map[string]bool)
		for _, q := range searchQueries {
			torrents, err := nyaaClient.Search(ctx, q)
			if err != nil {
				continue
			}
			for _, t := range torrents {
				mag := t.Magnet()
				if visitedMagnets[mag] {
					continue
				}
				if matchNyaaTorrent(t.Title, epNum) {
					visitedMagnets[mag] = true
					mu.Lock()
					results = append(results, PlayableTorrent{
						Source:    "Nyaa.si",
						Title:     t.Title,
						Size:      t.Size,
						Seeders:   t.Seeders,
						Magnet:    mag,
						Voiceover: "Original",
					})
					mu.Unlock()
				}
			}
		}
	}()

	// 3. Поиск на Rutor
	wg.Add(1)
	go func() {
		defer wg.Done()
		rutorClient := rutor.NewClient(15 * time.Second)
		searchQueries := []string{animeTitleEn}
		if query != animeTitleEn && query != "" {
			searchQueries = append(searchQueries, query)
		}

		visitedMagnets := make(map[string]bool)
		for _, q := range searchQueries {
			torrents, err := rutorClient.Search(ctx, q)
			if err != nil {
				continue
			}
			for _, t := range torrents {
				if visitedMagnets[t.Magnet] {
					continue
				}
				if matchRutorTorrent(t.Title, epNum) {
					visitedMagnets[t.Magnet] = true
					vos := detectVoiceovers(t.Title)
					if len(vos) == 0 {
						mu.Lock()
						results = append(results, PlayableTorrent{
							Source:    "Rutor",
							Title:     t.Title,
							Size:      t.Size,
							Seeders:   t.Seeders,
							Magnet:    t.Magnet,
							Voiceover: "Other",
						})
						mu.Unlock()
					} else {
						for _, vo := range vos {
							mu.Lock()
							results = append(results, PlayableTorrent{
								Source:    "Rutor",
								Title:     t.Title,
								Size:      t.Size,
								Seeders:   t.Seeders,
								Magnet:    t.Magnet,
								Voiceover: vo,
							})
							mu.Unlock()
						}
					}
				}
			}
		}
	}()

	wg.Wait()
	return results
}

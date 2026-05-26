package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/WhymeUMR/ani/internal/anilibria"
	"github.com/WhymeUMR/ani/internal/jikan"
	"github.com/WhymeUMR/ani/internal/torrent"
)

// playTorrentFlow запускает интерактивный флоу поиска и проигрывания аниме
func playTorrentFlow(ctx context.Context, query string) error {
	jClient := jikan.NewClient("", 10*time.Second)
	fmt.Printf("\n%sSearching catalog for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)

	animeList, err := jClient.SearchAnime(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search Jikan: %w", err)
	}

	if len(animeList) == 0 {
		// Автоматический мост для русскоязычных запросов:
		// Если Jikan (MAL) ничего не нашел, пробуем разрешить русское название через AniLibria
		fmt.Printf("%sNo results on MyAnimeList. Trying to resolve Russian title via AniLibria...%s\n", colorGray, colorReset)
		alClient := anilibria.NewClient(10 * time.Second)
		releases, err := alClient.Search(ctx, query)
		if err == nil && len(releases) > 0 {
			// Нашли соответствующее английское название
			resolvedTitle := releases[0].Name.English
			if resolvedTitle != "" {
				fmt.Printf("%sResolved title: %s%q%s. Searching on MyAnimeList...%s\n\n",
					colorGray, colorYellow, resolvedTitle, colorGray, colorReset)

				animeList, err = jClient.SearchAnime(ctx, resolvedTitle)
				if err != nil {
					return fmt.Errorf("failed to search Jikan with resolved title: %w", err)
				}
			}
		}
	}

	if len(animeList) == 0 {
		fmt.Printf("%sNo anime found for %q.%s\n", colorYellow, query, colorReset)
		return nil
	}

	// 1. Выбираем аниме из результатов поиска
	printTable(animeList)
	animeChoice, err := readChoice("\nSelect anime number: ", len(animeList))
	if err != nil {
		return err
	}
	selectedAnime := animeList[animeChoice-1]

	// 2. Выбираем серию
	episodesCount := selectedAnime.Episodes
	epNum := 1
	if episodesCount == 0 || episodesCount > 1 {
		maxEpStr := "unknown"
		if episodesCount > 0 {
			maxEpStr = strconv.Itoa(episodesCount)
		}

		fmt.Printf("\n%sAnime has %s episodes.%s\n", colorCyan, maxEpStr, colorReset)
		prompt := fmt.Sprintf("Enter episode number to watch (1-%s) [default 1]: ", maxEpStr)

		reader := bufio.NewReader(os.Stdin)
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			epNum = 1
		} else {
			val, err := strconv.Atoi(input)
			if err != nil || val < 1 || (episodesCount > 0 && val > episodesCount) {
				return fmt.Errorf("invalid episode number")
			}
			epNum = val
		}
	}

	// 3. Запускаем параллельный поиск по всем источникам
	fmt.Printf("\n%sSearching torrents for episode %d on all sources...%s\n", colorCyan, epNum, colorReset)

	searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	torrents := searchAllSources(searchCtx, query, selectedAnime.Title, epNum)

	if len(torrents) == 0 {
		fmt.Printf("\n%sNo torrents containing episode %d found on AniLibria, Nyaa.si or Rutor.%s\n", colorRed, epNum, colorReset)
		return nil
	}

	// Группируем торренты по озвучкам
	voiceoverMap := make(map[string][]PlayableTorrent)
	for _, t := range torrents {
		voiceoverMap[t.Voiceover] = append(voiceoverMap[t.Voiceover], t)
	}

	// Задаем порядок вывода озвучек
	voiceoverOrder := []struct {
		Key  string
		Name string
	}{
		{"Original", "Оригинал + Субтитры (Nyaa.si)"},
		{"AniLibria", "AniLibria (Русская озвучка)"},
		{"Studio Band", "Studio Band (Русская озвучка)"},
		{"JAM Club", "JAM Club / Kansai (Русская озвучка)"},
		{"AniDUB", "AniDUB (Русская озвучка)"},
		{"SHIZA Project", "SHIZA Project (Русская озвучка)"},
		{"AnimeVost", "AnimeVost (Русская озвучка)"},
		{"steponee", "steponee (Русская озвучка)"},
		{"Other", "Другие русские озвучки / Раздачи"},
	}

	type AvailableVoiceover struct {
		Key   string
		Name  string
		Items []PlayableTorrent
	}
	var available []AvailableVoiceover
	for _, vo := range voiceoverOrder {
		if items, ok := voiceoverMap[vo.Key]; ok && len(items) > 0 {
			available = append(available, AvailableVoiceover{
				Key:   vo.Key,
				Name:  vo.Name,
				Items: items,
			})
		}
	}

	if len(available) == 0 {
		fmt.Printf("\n%sNo available voiceovers found for episode %d.%s\n", colorRed, epNum, colorReset)
		return nil
	}

	// 4. Выбор озвучки пользователем
	fmt.Printf("\n%sAvailable voiceovers for episode %d:%s\n", colorYellow, epNum, colorReset)
	for i, av := range available {
		fmt.Printf("  [%d] %s (%d torrents)\n", i+1, av.Name, len(av.Items))
	}

	voChoice, err := readChoice("\nSelect voiceover number: ", len(available))
	if err != nil {
		return err
	}
	selectedVO := available[voChoice-1]

	// 5. Автоматически выбираем лучшую раздачу под капотом
	selectedTorrent := selectBestTorrent(selectedVO.Items)
	fmt.Printf("\n%sAutomatically selected best torrent: %s (%s, %d seeds)%s\n",
		colorGreen, truncate(selectedTorrent.Title, 70), selectedTorrent.Size, selectedTorrent.Seeders, colorReset)

	// 6. Запускаем стриминг и автоматический выбор файла серии
	return streamMagnetFlow(selectedTorrent.Magnet, selectedTorrent.Title, epNum)
}

// streamMagnetFlow инициализирует торрент-стример и запускает плеер
func streamMagnetFlow(magnet string, title string, epNum int) error {
	fmt.Printf("\n%sStarting torrent streamer... (fetching metadata)%s\n", colorCyan, colorReset)

	streamer, err := torrent.NewStreamer("")
	if err != nil {
		return fmt.Errorf("failed to initialize torrent streamer: %w", err)
	}
	defer streamer.Close()

	files, err := streamer.LoadMagnet(magnet)
	if err != nil {
		return fmt.Errorf("failed to load torrent metadata: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in torrent")
	}

	selectedFileIndex := 0
	if len(files) > 1 {
		autoIdx := findEpisodeFile(files, epNum)
		if autoIdx >= 0 {
			selectedFileIndex = autoIdx
			fmt.Printf("\n%sAutomatically detected file for episode %d: %s%s%s (%s)\n",
				colorGreen, epNum, colorYellow, files[selectedFileIndex].Name, colorReset,
				fmt.Sprintf("%.2f GiB", float64(files[selectedFileIndex].Size)/(1024*1024*1024)))
		} else {
			fmt.Printf("\n%sMultiple files found in torrent, please choose:%s\n", colorCyan, colorReset)
			fw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(fw, "%s#\tFILE NAME\tSIZE%s\n", colorCyan, colorReset)
			for i, f := range files {
				fmt.Fprintf(fw, "%s[%d]\t%s%s\t%s%.2f GiB%s\n",
					colorCyan, i+1,
					colorYellow, truncate(f.Name, 60),
					colorReset, float64(f.Size)/(1024*1024*1024),
					colorReset,
				)
			}
			fw.Flush()

			fileChoice, err := readChoice("\nSelect file number to play: ", len(files))
			if err != nil {
				return err
			}
			selectedFileIndex = fileChoice - 1
		}
	}

	fmt.Printf("\n%sStarting local stream for %s%s%s...\n", colorCyan, colorYellow, files[selectedFileIndex].Name, colorCyan)
	streamURL, err := streamer.StartStreaming(selectedFileIndex)
	if err != nil {
		return fmt.Errorf("failed to start streaming: %w", err)
	}

	return launchPlayer(streamURL)
}

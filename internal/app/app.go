package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/WhymeUMR/ani/internal/anilibria"
	"github.com/WhymeUMR/ani/internal/jikan"
	"github.com/WhymeUMR/ani/internal/nyaa"
	"github.com/WhymeUMR/ani/internal/rutor"
	"github.com/WhymeUMR/ani/internal/torrent"
)

const logo = `
  ▄████████ ███▄▄▄▄    ▄█  
  ███    ███ ███▀▀▀██▄ ███  
  ███    ███ ███   ███ ███▌ 
  ███    ███ ███   ███ ███▌ 
▀███████████ ███   ███ ███▌ 
  ███    ███ ███   ███ ███  
  ███    ███ ███   ███ ███  
  ███    █▀   ▀█   █▀  █▀  
`

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[1;36m"
	colorYellow = "\033[1;33m"
	colorGreen  = "\033[1;32m"
	colorRed    = "\033[1;31m"
	colorGray   = "\033[0;90m"
)

func Run(args []string) error {
	fmt.Print(logo)

	client := jikan.NewClient("", 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if len(args) == 0 {
		showHelp()
		fmt.Printf("\n%sFetching popular anime...%s\n\n", colorCyan, colorReset)
		return fetchAndPrintTop(ctx, client)
	}

	command := args[0]
	switch command {
	case "help", "-h", "--help":
		showHelp()
		return nil
	case "top":
		fmt.Printf("\n%sFetching popular anime...%s\n\n", colorCyan, colorReset)
		return fetchAndPrintTop(ctx, client)
	case "search":
		if len(args) < 2 {
			return fmt.Errorf("search command requires a query: ani search <anime name>")
		}
		query := strings.Join(args[1:], " ")
		fmt.Printf("\n%sSearching for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)
		return searchAndPrint(ctx, client, query)
	case "play":
		if len(args) < 2 {
			return fmt.Errorf("play command requires a query: ani play <anime name>")
		}
		query := strings.Join(args[1:], " ")
		return playTorrentFlow(ctx, query)
	default:
		// If unknown command, treat all arguments as a search query
		query := strings.Join(args, " ")
		fmt.Printf("\n%sSearching for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)
		return searchAndPrint(ctx, client, query)
	}
}

func showHelp() {
	fmt.Printf("%sUsage:%s\n", colorYellow, colorReset)
	fmt.Println("  ani                    Show popular anime")
	fmt.Println("  ani top                Show popular anime")
	fmt.Println("  ani search <query>     Search for anime by name")
	fmt.Println("  ani play <query>       Search and play torrent (AniLibria / Nyaa.si / Rutor)")
	fmt.Println("  ani <query>            Quick search for anime")
}

func fetchAndPrintTop(ctx context.Context, client *jikan.Client) error {
	list, err := client.GetTopAnime(ctx)
	if err != nil {
		return fmt.Errorf("%serror fetching top anime: %v%s", colorRed, err, colorReset)
	}
	printTable(list)
	return nil
}

func searchAndPrint(ctx context.Context, client *jikan.Client, query string) error {
	list, err := client.SearchAnime(ctx, query)
	if err != nil {
		return fmt.Errorf("%serror searching anime: %v%s", colorRed, err, colorReset)
	}
	if len(list) == 0 {
		fmt.Printf("%sNo anime found for query %q.%s\n", colorYellow, query, colorReset)
		return nil
	}
	printTable(list)
	return nil
}

func playTorrentFlow(ctx context.Context, query string) error {
	jClient := jikan.NewClient("", 10*time.Second)
	fmt.Printf("\n%sSearching catalog for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)

	animeList, err := jClient.SearchAnime(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search Jikan: %w", err)
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

	// 5. Выбор конкретной раздачи (если их несколько для выбранной озвучки)
	var selectedTorrent PlayableTorrent
	if len(selectedVO.Items) == 1 {
		selectedTorrent = selectedVO.Items[0]
		fmt.Printf("\n%sAutomatically selected torrent: %s (%s, %d seeds)%s\n",
			colorGreen, selectedTorrent.Title, selectedTorrent.Size, selectedTorrent.Seeders, colorReset)
	} else {
		fmt.Printf("\n%sAvailable torrents for %s:%s\n", colorYellow, selectedVO.Name, colorReset)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "%s#\tSIZE\tSEEDERS\tTITLE%s\n", colorCyan, colorReset)
		for i, t := range selectedVO.Items {
			fmt.Fprintf(w, "%s[%d]\t%s%s\t%s%d\t%s%s%s\n",
				colorCyan, i+1,
				colorReset, t.Size,
				colorGreen, t.Seeders,
				colorYellow, truncate(t.Title, 80),
				colorReset,
			)
		}
		w.Flush()

		tChoice, err := readChoice("\nSelect torrent version: ", len(selectedVO.Items))
		if err != nil {
			return err
		}
		selectedTorrent = selectedVO.Items[tChoice-1]
	}

	// 6. Запускаем стриминг и автоматический выбор файла серии
	return streamMagnetFlow(selectedTorrent.Magnet, selectedTorrent.Title, epNum)
}

type PlayableTorrent struct {
	Source    string
	Title     string
	Size      string
	Seeders   int
	Magnet    string
	Voiceover string
}

var (
	// Паттерны для поиска диапазонов серий, например [01-12], (01-12), серии 1-12
	rangeRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?:сери[ия]|эпизод[ы]?|сезон)?\s*\[?\b(\d+)\s*[-–~]\s*(\d+)\b`),
		regexp.MustCompile(`\b(\d+)\s*[-–~]\s*(\d+)\s*(?:из)?`),
	}

	// Паттерны для поиска одиночных серий, например [05 из 12], серия 5
	singleRegexes = []*regexp.Regexp{
		regexp.MustCompile(`\[?\s*(\d+)\s+из`),
		regexp.MustCompile(`(?:сери[яи]|эпизод)\s*(\d+)`),
	}

	// Паттерны для Nyaa.si одиночных серий, например "- 05", "ep05"
	nyaaRegexes = []*regexp.Regexp{
		regexp.MustCompile(`\s+-\s+0*(\d+)\b`),
		regexp.MustCompile(`\bep(?:isode)?\s*0*(\d+)\b`),
		regexp.MustCompile(`\b0*(\d+)\s*(?:v\d+)?\s*\[`),
	}
)

// matchAniLibriaTorrent проверяет, входит ли серия в описание торрента AniLibria
func matchAniLibriaTorrent(desc string, epNum int) bool {
	desc = strings.ToLower(desc)
	if desc == "" {
		return epNum == 1
	}

	for _, re := range rangeRegexes {
		matches := re.FindStringSubmatch(desc)
		if len(matches) >= 3 {
			start, _ := strconv.Atoi(matches[1])
			end, _ := strconv.Atoi(matches[2])
			if epNum >= start && epNum <= end {
				return true
			}
		}
	}

	for _, re := range singleRegexes {
		matches := re.FindStringSubmatch(desc)
		if len(matches) >= 2 {
			single, _ := strconv.Atoi(matches[1])
			if epNum == single {
				return true
			}
		}
	}

	if val, err := strconv.Atoi(strings.TrimSpace(desc)); err == nil {
		return val == epNum
	}

	return false
}

// matchNyaaTorrent проверяет, подходит ли торрент Nyaa.si для серии
func matchNyaaTorrent(title string, epNum int) bool {
	title = strings.ToLower(title)

	// Если это пак всего сезона, он содержит все серии
	if strings.Contains(title, "batch") || strings.Contains(title, "complete") || strings.Contains(title, "season") || strings.Contains(title, "01-") || strings.Contains(title, "01~") {
		return true
	}

	for _, re := range rangeRegexes {
		matches := re.FindStringSubmatch(title)
		if len(matches) >= 3 {
			start, _ := strconv.Atoi(matches[1])
			end, _ := strconv.Atoi(matches[2])
			if epNum >= start && epNum <= end {
				return true
			}
		}
	}

	for _, re := range nyaaRegexes {
		matches := re.FindStringSubmatch(title)
		if len(matches) >= 2 {
			single, _ := strconv.Atoi(matches[1])
			if epNum == single {
				return true
			}
		}
	}

	if epNum == 1 {
		return true
	}

	return false
}

// matchRutorTorrent проверяет, подходит ли торрент Rutor для серии
func matchRutorTorrent(title string, epNum int) bool {
	title = strings.ToLower(title)

	for _, re := range rangeRegexes {
		matches := re.FindStringSubmatch(title)
		if len(matches) >= 3 {
			start, _ := strconv.Atoi(matches[1])
			end, _ := strconv.Atoi(matches[2])
			if epNum >= start && epNum <= end {
				return true
			}
		}
	}

	for _, re := range singleRegexes {
		matches := re.FindStringSubmatch(title)
		if len(matches) >= 2 {
			single, _ := strconv.Atoi(matches[1])
			if epNum == single {
				return true
			}
		}
	}

	// Фильмы, OVA и спешлы
	if epNum == 1 {
		if strings.Contains(title, "movie") || strings.Contains(title, "фильм") || strings.Contains(title, "special") || strings.Contains(title, "ova") || strings.Contains(title, "ова") || strings.Contains(title, "спешл") {
			return true
		}
		if !strings.Contains(title, "[") && !strings.Contains(title, "(") {
			return true
		}
	}

	return false
}

// detectVoiceovers определяет все студии озвучки по названию раздачи
func detectVoiceovers(title string) []string {
	title = strings.ToLower(title)
	var vos []string

	if strings.Contains(title, "studio band") || strings.Contains(title, "studioband") || strings.Contains(title, "студийная банда") {
		vos = append(vos, "Studio Band")
	}
	if strings.Contains(title, "jam") || strings.Contains(title, "kansai") || strings.Contains(title, "кансай") {
		vos = append(vos, "JAM Club")
	}
	if strings.Contains(title, "anidub") || strings.Contains(title, "анидаб") {
		vos = append(vos, "AniDUB")
	}
	if strings.Contains(title, "shiza project") || strings.Contains(title, "shiza") || strings.Contains(title, "шиза") || strings.Contains(title, "shizaproject") {
		vos = append(vos, "SHIZA Project")
	}
	if strings.Contains(title, "animevost") || strings.Contains(title, "anime vost") || strings.Contains(title, "анимевост") {
		vos = append(vos, "AnimeVost")
	}
	if strings.Contains(title, "steponee") || strings.Contains(title, "step onee") {
		vos = append(vos, "steponee")
	}

	return vos
}

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

// findEpisodeFile ищет индекс файла, соответствующего серии, по маске названия
func findEpisodeFile(files []torrent.TorrentFile, ep int) int {
	epStrLong := fmt.Sprintf("%02d", ep)
	epStrShort := fmt.Sprintf("%d", ep)

	// Паттерны поиска номера серии с границами слов
	patterns := []string{
		"s01e" + epStrLong,
		"s02e" + epStrLong,
		"s03e" + epStrLong,
		"e" + epStrLong,
		"e" + epStrShort,
		"ep" + epStrLong,
		"ep" + epStrShort,
		"ep." + epStrLong,
		"ep." + epStrShort,
		"episode " + epStrLong,
		"episode " + epStrShort,
		" " + epStrLong + " ",
		" " + epStrShort + " ",
		"-" + epStrLong,
		"_" + epStrLong,
		"[" + epStrLong + "]",
		"[" + epStrShort + "]",
		" - " + epStrLong,
		" - " + epStrShort,
		" " + epStrLong + ".",
		" " + epStrShort + ".",
		"_" + epStrLong + ".",
	}

	var candidates []int
	for i, f := range files {
		nameLower := strings.ToLower(f.Name)
		if !isVideoFile(nameLower) {
			continue
		}

		for _, pat := range patterns {
			if strings.Contains(nameLower, pat) {
				candidates = append(candidates, i)
				break
			}
		}
	}

	if len(candidates) == 1 {
		return candidates[0]
	}

	return -1
}

func isVideoFile(name string) bool {
	exts := []string{".mkv", ".mp4", ".avi", ".mov", ".flv", ".webm", ".ts", ".m4v"}
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

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

func readChoice(prompt string, maxVal int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)
		val, err := strconv.Atoi(input)
		if err != nil || val < 1 || val > maxVal {
			fmt.Printf("%sInvalid choice. Please enter a number between 1 and %d.%s\n", colorRed, maxVal, colorReset)
			continue
		}
		return val, nil
	}
}

func launchPlayer(streamURL string) error {
	if _, err := exec.LookPath("mpv"); err == nil {
		fmt.Printf("%sLaunching mpv video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command("mpv", streamURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	vlcMacPath := "/Applications/VLC.app/Contents/MacOS/VLC"
	if _, err := os.Stat(vlcMacPath); err == nil {
		fmt.Printf("%sLaunching VLC video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command(vlcMacPath, streamURL)
		return cmd.Run()
	}

	if _, err := exec.LookPath("vlc"); err == nil {
		fmt.Printf("%sLaunching VLC video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command("vlc", streamURL)
		return cmd.Run()
	}

	fmt.Printf("\n%sCould not automatically find mpv or VLC installed in PATH.%s\n", colorYellow, colorReset)
	fmt.Printf("%sPlease open this stream URL in your favorite media player (e.g. VLC, IINA):%s\n", colorCyan, colorReset)
	fmt.Printf("%s%s%s\n\n", colorGreen, streamURL, colorReset)
	fmt.Println("Press Enter to stop streaming and exit...")

	var input string
	fmt.Scanln(&input)
	return nil
}

func printTable(list []jikan.Anime) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%sID\tTITLE\tSCORE\tTYPE\tEPISODES\tSTATUS\tYEAR%s\n", colorCyan, colorReset)

	for _, item := range list {
		yearStr := "-"
		if item.Year > 0 {
			yearStr = fmt.Sprintf("%d", item.Year)
		}

		scoreStr := fmt.Sprintf("%.2f", item.Score)
		if item.Score == 0 {
			scoreStr = "-"
		}

		fmt.Fprintf(w, "%s%d\t%s%s\t%s%s\t%s%s\t%s%d\t%s%s\t%s%s%s\n",
			colorGray, item.ID,
			colorYellow, truncate(item.Title, 40),
			colorGreen, scoreStr,
			colorReset, item.Type,
			colorReset, item.Episodes,
			colorReset, item.Status,
			colorGray, yearStr,
			colorReset,
		)
	}
	w.Flush()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

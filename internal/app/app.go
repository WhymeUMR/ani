package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
	fmt.Printf("\n%sSelect Torrent Source:%s\n", colorYellow, colorReset)
	fmt.Println("  [1] AniLibria (Russian voiceover / Русская озвучка)")
	fmt.Println("  [2] Nyaa.si (Original audio with subs / Оригинал + субтитры)")
	fmt.Println("  [3] Rutor (Diverse RU Voiceovers: Studio Band, JAM, AniDUB, steponee, etc.)")

	sourceChoice, err := readChoice("\nEnter source number: ", 3)
	if err != nil {
		return err
	}

	switch sourceChoice {
	case 1:
		return playAniLibria(ctx, query)
	case 2:
		return playNyaa(ctx, query)
	case 3:
		return playRutor(ctx, query)
	default:
		return fmt.Errorf("invalid source choice")
	}
}

func playAniLibria(ctx context.Context, query string) error {
	alClient := anilibria.NewClient(15 * time.Second)
	fmt.Printf("\n%sSearching catalog on AniLibria for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)

	releases, err := alClient.Search(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search AniLibria: %w", err)
	}

	if len(releases) == 0 {
		fmt.Printf("%sNo releases found on AniLibria for %q%s\n", colorYellow, query, colorReset)
		return nil
	}

	// Display releases
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s#\tTITLE (RU)\tTITLE (EN)\tYEAR%s\n", colorCyan, colorReset)
	limit := 10
	if len(releases) < limit {
		limit = len(releases)
	}

	for i := 0; i < limit; i++ {
		r := releases[i]
		fmt.Fprintf(w, "%s[%d]\t%s%s\t%s%s\t%s%d%s\n",
			colorCyan, i+1,
			colorYellow, truncate(r.Name.Main, 45),
			colorReset, truncate(r.Name.English, 35),
			colorGray, r.Year,
			colorReset,
		)
	}
	w.Flush()

	choice, err := readChoice("\nSelect anime number: ", limit)
	if err != nil {
		return err
	}

	selectedRelease := releases[choice-1]
	fmt.Printf("\n%sLoading release details for: %s%s...\n", colorCyan, colorYellow, selectedRelease.Name.Main)

	// Fetch detailed info with torrents
	detail, err := alClient.GetRelease(ctx, selectedRelease.ID)
	if err != nil {
		return fmt.Errorf("failed to load release details: %w", err)
	}

	if len(detail.Torrents) == 0 {
		fmt.Printf("%sNo torrents found on AniLibria for %q%s\n", colorYellow, selectedRelease.Name.Main, colorReset)
		return nil
	}

	// Display torrents
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "%s#\tQUALITY\tEPISODES\tSIZE\tSEEDERS\tLABEL%s\n", colorCyan, colorReset)
	for i, t := range detail.Torrents {
		fmt.Fprintf(tw, "%s[%d]\t%s%s\t%s%s\t%s%.2f GiB\t%s%d\t%s%s%s\n",
			colorCyan, i+1,
			colorGreen, t.Quality.Value,
			colorYellow, t.Description,
			colorReset, float64(t.Size)/(1024*1024*1024),
			colorGreen, t.Seeders,
			colorGray, truncate(t.Label, 45),
			colorReset,
		)
	}
	tw.Flush()

	torrentChoice, err := readChoice("\nSelect torrent quality/version: ", len(detail.Torrents))
	if err != nil {
		return err
	}

	selectedTorrent := detail.Torrents[torrentChoice-1]
	return streamMagnet(selectedTorrent.Magnet, selectedTorrent.Label)
}

func playNyaa(ctx context.Context, query string) error {
	nyaaClient := nyaa.NewClient(15 * time.Second)
	fmt.Printf("\n%sSearching torrents for %s%q%s on Nyaa.si...\n\n", colorCyan, colorYellow, query, colorCyan)

	torrents, err := nyaaClient.Search(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search Nyaa: %w", err)
	}

	if len(torrents) == 0 {
		fmt.Printf("%sNo torrents found on Nyaa.si for %q%s\n", colorYellow, query, colorReset)
		return nil
	}

	// Display torrents list
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s#\tTITLE\tSIZE\tSEEDERS%s\n", colorCyan, colorReset)
	limit := 10
	if len(torrents) < limit {
		limit = len(torrents)
	}

	for i := 0; i < limit; i++ {
		t := torrents[i]
		fmt.Fprintf(w, "%s[%d]\t%s%s\t%s%s\t%s%d%s\n",
			colorCyan, i+1,
			colorYellow, truncate(t.Title, 60),
			colorReset, t.Size,
			colorGreen, t.Seeders,
			colorReset,
		)
	}
	w.Flush()

	choice, err := readChoice("\nSelect torrent number: ", limit)
	if err != nil {
		return err
	}

	selectedTorrent := torrents[choice-1]
	return streamMagnet(selectedTorrent.Magnet(), selectedTorrent.Title)
}

func playRutor(ctx context.Context, query string) error {
	rutorClient := rutor.NewClient(15 * time.Second)
	fmt.Printf("\n%sSearching torrents on Rutor for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)

	torrents, err := rutorClient.Search(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to search Rutor: %w", err)
	}

	if len(torrents) == 0 {
		fmt.Printf("%sNo torrents found on Rutor for %q%s\n", colorYellow, query, colorReset)
		return nil
	}

	// Display torrents
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s#\tSIZE\tSEEDERS\tTITLE (VOICEOVER)%s\n", colorCyan, colorReset)
	limit := 15
	if len(torrents) < limit {
		limit = len(torrents)
	}

	for i := 0; i < limit; i++ {
		t := torrents[i]
		fmt.Fprintf(w, "%s[%d]\t%s%s\t%s%d\t%s%s%s\n",
			colorCyan, i+1,
			colorReset, t.Size,
			colorGreen, t.Seeders,
			colorYellow, truncate(t.Title, 75),
			colorReset,
		)
	}
	w.Flush()

	choice, err := readChoice("\nSelect torrent number to play: ", limit)
	if err != nil {
		return err
	}

	selectedTorrent := torrents[choice-1]
	return streamMagnet(selectedTorrent.Magnet, selectedTorrent.Title)
}

func streamMagnet(magnet string, title string) error {
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
		fmt.Printf("\n%sMultiple files found in torrent:%s\n", colorCyan, colorReset)
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
	// 1. Try mpv from PATH
	if _, err := exec.LookPath("mpv"); err == nil {
		fmt.Printf("%sLaunching mpv video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command("mpv", streamURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// 2. Try VLC in default Mac applications path
	vlcMacPath := "/Applications/VLC.app/Contents/MacOS/VLC"
	if _, err := os.Stat(vlcMacPath); err == nil {
		fmt.Printf("%sLaunching VLC video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command(vlcMacPath, streamURL)
		return cmd.Run()
	}

	// 3. Try VLC from PATH
	if _, err := exec.LookPath("vlc"); err == nil {
		fmt.Printf("%sLaunching VLC video player...%s\n", colorCyan, colorReset)
		cmd := exec.Command("vlc", streamURL)
		return cmd.Run()
	}

	// Fallback: print stream URL and wait for Enter
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

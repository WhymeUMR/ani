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

	"github.com/WhymeUMR/ani/internal/jikan"
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

// Run является основной точкой входа для CLI приложения ani
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
		// Если команда неизвестна, считаем все аргументы поисковым запросом
		query := strings.Join(args, " ")
		fmt.Printf("\n%sSearching for %s%q%s...\n\n", colorCyan, colorYellow, query, colorCyan)
		return searchAndPrint(ctx, client, query)
	}
}

// showHelp выводит справочную информацию по командам
func showHelp() {
	fmt.Printf("%sUsage:%s\n", colorYellow, colorReset)
	fmt.Println("  ani                    Show popular anime")
	fmt.Println("  ani top                Show popular anime")
	fmt.Println("  ani search <query>     Search for anime by name")
	fmt.Println("  ani play <query>       Search and play torrent (AniLibria / Nyaa.si / Rutor)")
	fmt.Println("  ani <query>            Quick search for anime")
}

// fetchAndPrintTop получает топ аниме и выводит его в терминал
func fetchAndPrintTop(ctx context.Context, client *jikan.Client) error {
	list, err := client.GetTopAnime(ctx)
	if err != nil {
		return fmt.Errorf("%serror fetching top anime: %v%s", colorRed, err, colorReset)
	}
	printTable(list)
	return nil
}

// searchAndPrint выполняет поиск аниме и выводит результаты
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

// readChoice считывает выбор пользователя из консоли с валидацией границ
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

// launchPlayer запускает mpv или VLC на macOS с переданным streamURL
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

// printTable выводит список аниме в красивом табличном виде
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

// truncate обрезает строку до максимальной длины, добавляя многоточие
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

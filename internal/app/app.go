package app

import (
	"context"
	"fmt"
	"os"
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

func Run(args []string) error {
	fmt.Print(logo)

	client := jikan.NewClient("", 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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

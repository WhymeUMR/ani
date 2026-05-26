package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/WhymeUMR/ani/internal/torrent"
)

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

// isVideoFile проверяет, является ли файл видеофайлом по его расширению
func isVideoFile(name string) bool {
	exts := []string{".mkv", ".mp4", ".avi", ".mov", ".flv", ".webm", ".ts", ".m4v"}
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

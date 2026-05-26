package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/WhymeUMR/ani/internal/anilibria"
	"github.com/WhymeUMR/ani/internal/torrent"
)

func TestMatchAniLibriaTorrent(t *testing.T) {
	tests := []struct {
		desc  string
		ep    int
		match bool
	}{
		{"01-12", 5, true},
		{"01-12", 13, false},
		{"01-12 из 12", 1, true},
		{"12 из 12", 12, true},
		{"12 из 12", 5, false},
		{"05", 5, true},
		{"5", 5, true},
		{"серия 05", 5, true},
		{"серии 01-28", 28, true},
		{"серии 01-28", 29, false},
		{"", 1, true},
		{"", 2, false},
	}

	for _, tt := range tests {
		res := matchAniLibriaTorrent(tt.desc, tt.ep)
		if res != tt.match {
			t.Errorf("matchAniLibriaTorrent(%q, %d) = %v; want %v", tt.desc, tt.ep, res, tt.match)
		}
	}
}

func TestMatchNyaaTorrent(t *testing.T) {
	tests := []struct {
		title string
		ep    int
		match bool
	}{
		{"[Erai-raws] Sousou no Frieren - 05 [1080p].mkv", 5, true},
		{"[Erai-raws] Sousou no Frieren - 05 [1080p].mkv", 6, false},
		{"[SubsPlease] Chainsaw Man - 12 (1080p) [F124A5]", 12, true},
		{"Chainsaw Man Season 1 [Batch] [1080p]", 5, true},
		{"Chainsaw Man S01 Complete (01-12)", 3, true},
		{"[Erai-raws] Frieren ep 05 [1080p]", 5, true},
		{"Frieren 05 [1080p]", 5, true},
	}

	for _, tt := range tests {
		res := matchNyaaTorrent(tt.title, tt.ep)
		if res != tt.match {
			t.Errorf("matchNyaaTorrent(%q, %d) = %v; want %v", tt.title, tt.ep, res, tt.match)
		}
	}
}

func TestMatchRutorTorrent(t *testing.T) {
	tests := []struct {
		title string
		ep    int
		match bool
	}{
		{"Провожающая в последний путь Фрирен [01-28 из 28] (2023) | Studio Band", 5, true},
		{"Провожающая в последний путь Фрирен [01-28 из 28] (2023) | Studio Band", 29, false},
		{"Клинок, рассекающий демонов [05 из 26] (2019) | AniDUB", 5, true},
		{"Клинок, рассекающий демонов [05 из 26] (2019) | AniDUB", 6, false},
		{"Унесенные призраками / Sen to Chihiro no Kamikakushi (2001) BDRip 1080p [Фильм]", 1, true},
		{"Унесенные призраками / Sen to Chihiro no Kamikakushi (2001) BDRip 1080p [Фильм]", 2, false},
	}

	for _, tt := range tests {
		res := matchRutorTorrent(tt.title, tt.ep)
		if res != tt.match {
			t.Errorf("matchRutorTorrent(%q, %d) = %v; want %v", tt.title, tt.ep, res, tt.match)
		}
	}
}

func TestDetectVoiceovers(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{"Frieren [01-28] | Studio Band", []string{"Studio Band"}},
		{"Frieren [01-28] | StudioBand", []string{"Studio Band"}},
		{"Frieren [01-28] | Студийная банда", []string{"Studio Band"}},
		{"Frieren [01-28] | JAM Club", []string{"JAM Club"}},
		{"Frieren [01-28] | JAM-Club", []string{"JAM Club"}},
		{"Frieren [01-28] | AniDUB", []string{"AniDUB"}},
		{"Frieren [01-28] | Анидаб", []string{"AniDUB"}},
		{"Frieren [01-28] | SHIZA Project", []string{"SHIZA Project"}},
		{"Frieren [01-28] | Шиза", []string{"SHIZA Project"}},
		{"Frieren [01-28] | AnimeVost", []string{"AnimeVost"}},
		{"Frieren [01-28] | Анимевост", []string{"AnimeVost"}},
		{"Frieren [01-28] | steponee", []string{"steponee"}},
		{"Ванпанчмен / One Punch Man | Kansai, SHIZA project", []string{"JAM Club", "SHIZA Project"}},
		{"Ванпанчмен / One Punch Man | KANSAI, HDRezka", []string{"JAM Club"}},
		{"Ванпанчмен / One-Punch Man | кансай", []string{"JAM Club"}},
		{"Classroom of the Elite | StudioBand, JAMClub", []string{"Studio Band", "JAM Club"}},
		{"Frieren [01-28] | Unknown Voice", nil},
	}

	for _, tt := range tests {
		res := detectVoiceovers(tt.title)
		if len(res) != len(tt.want) {
			t.Fatalf("detectVoiceovers(%q) returned %d items, want %d", tt.title, len(res), len(tt.want))
		}
		for i, v := range res {
			if v != tt.want[i] {
				t.Errorf("detectVoiceovers(%q)[%d] = %q; want %q", tt.title, i, v, tt.want[i])
			}
		}
	}
}

func TestFindEpisodeFile(t *testing.T) {
	files := []torrent.TorrentFile{
		{Index: 0, Name: "Chainsaw Man - 01.mkv", Size: 1000},
		{Index: 1, Name: "Chainsaw Man - 02.mkv", Size: 1000},
		{Index: 2, Name: "Chainsaw Man - 05.mkv", Size: 1000},
		{Index: 3, Name: "Readme.txt", Size: 100},
	}

	// 1. Точное совпадение серии 5
	idx := findEpisodeFile(files, 5)
	if idx != 2 {
		t.Errorf("findEpisodeFile(..., 5) = %d; want 2", idx)
	}

	// 2. Точное совпадение серии 1
	idx = findEpisodeFile(files, 1)
	if idx != 0 {
		t.Errorf("findEpisodeFile(..., 1) = %d; want 0", idx)
	}

	// 3. Отсутствующая серия 3
	idx = findEpisodeFile(files, 3)
	if idx != -1 {
		t.Errorf("findEpisodeFile(..., 3) = %d; want -1", idx)
	}
}

func TestSelectBestTorrent(t *testing.T) {
	// 1. Выбор 1080p над 720p при равных сидерах
	torrents1 := []PlayableTorrent{
		{Title: "Chainsaw Man [05] 720p", Seeders: 10},
		{Title: "Chainsaw Man [05] 1080p", Seeders: 10},
	}
	best := selectBestTorrent(torrents1)
	if best.Title != "Chainsaw Man [05] 1080p" {
		t.Errorf("expected 1080p to be selected, got %q", best.Title)
	}

	// 2. Выбор 720p, если у 1080p 0 сидеров
	torrents2 := []PlayableTorrent{
		{Title: "Chainsaw Man [05] 720p", Seeders: 10},
		{Title: "Chainsaw Man [05] 1080p", Seeders: 0},
	}
	best = selectBestTorrent(torrents2)
	if best.Title != "Chainsaw Man [05] 720p" {
		t.Errorf("expected 720p to be selected due to 0 seeds on 1080p, got %q", best.Title)
	}

	// 3. Выбор раздачи с большим количеством сидеров в одной категории качества
	torrents3 := []PlayableTorrent{
		{Title: "Chainsaw Man [05] 1080p (R1)", Seeders: 5},
		{Title: "Chainsaw Man [05] 1080p (R2)", Seeders: 25},
	}
	best = selectBestTorrent(torrents3)
	if best.Title != "Chainsaw Man [05] 1080p (R2)" {
		t.Errorf("expected R2 with 25 seeds to be selected, got %q", best.Title)
	}

	// 4. Корректная обработка пустого списка
	best = selectBestTorrent(nil)
	if best.Title != "" {
		t.Errorf("expected empty torrent for nil list, got %q", best.Title)
	}
}

func TestRussianTitleResolution(t *testing.T) {
	// Интеграционный тест: проверяем, что русский запрос транслируется в латиницу через AniLibria
	alClient := anilibria.NewClient(10 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	releases, err := alClient.Search(ctx, "Класс превосходства")
	if err != nil {
		t.Skipf("Skipping integration test due to network/API error: %v", err)
	}

	if len(releases) == 0 {
		t.Fatalf("expected to resolve Russian title to at least one release on AniLibria")
	}

	resolvedTitle := releases[0].Name.English
	expectedSubstring := "Youkoso Jitsuryoku"
	
	if !strings.Contains(resolvedTitle, expectedSubstring) && !strings.Contains(releases[0].Name.Main, "превосходства") {
		t.Errorf("expected resolved English title to contain %q or Russian name to contain 'превосходства', got resolved: %q, main: %q",
			expectedSubstring, resolvedTitle, releases[0].Name.Main)
	}
}

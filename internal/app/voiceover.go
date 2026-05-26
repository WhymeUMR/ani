package app

import (
	"strings"
)

// PlayableTorrent представляет раздачу, привязанную к конкретной озвучке
type PlayableTorrent struct {
	Source    string
	Title     string
	Size      string
	Seeders   int
	Magnet    string
	Voiceover string
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

package app

import (
	"strings"
)

// selectBestTorrent автоматически выбирает лучшую раздачу на основе весов качества и количества сидеров
func selectBestTorrent(torrents []PlayableTorrent) PlayableTorrent {
	if len(torrents) == 0 {
		return PlayableTorrent{}
	}
	if len(torrents) == 1 {
		return torrents[0]
	}

	getQualityWeight := func(title string) int {
		title = strings.ToLower(title)
		if strings.Contains(title, "1080") || strings.Contains(title, "fhd") {
			return 3 // FullHD
		}
		if strings.Contains(title, "720") || strings.Contains(title, "hd") {
			return 2 // HD
		}
		if strings.Contains(title, "480") || strings.Contains(title, "sd") {
			return 1 // SD
		}
		return 2 // По умолчанию среднее качество
	}

	bestIdx := 0
	bestScore := -100.0

	for i, t := range torrents {
		qWeight := getQualityWeight(t.Title)

		seeds := t.Seeders
		if seeds < 0 {
			seeds = 0
		}

		seedScore := float64(seeds)
		// Насыщение сидеров после 30 (для плавного стриминга 30 сидеров вполне достаточно)
		if seedScore > 30 {
			seedScore = 30 + (seedScore-30)*0.1
		}

		// Сильный штраф для раздач без сидеров
		if seeds == 0 {
			seedScore = -50.0
		}

		score := float64(qWeight)*20.0 + seedScore

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return torrents[bestIdx]
}

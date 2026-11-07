package httpapi

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// weatherBlend décrit un calcul qui utilise le run GFS le plus récent alors
// qu'il est encore en téléchargement : ses échéances d'abord, puis celles du
// run complet précédent au-delà de SwitchAt.
type weatherBlend struct {
	LatestRun   string
	FallbackRun string
	Downloaded  int
	Expected    int
	SwitchAt    time.Time
}

// weatherBlendResponse est envoyé à l'interface pour qu'elle signale la bascule.
type weatherBlendResponse struct {
	LatestRun    string `json:"latestRun"`
	FallbackRun  string `json:"fallbackRun"`
	Downloaded   int    `json:"downloaded"`
	Expected     int    `json:"expected"`
	SwitchAt     string `json:"switchAt"`
	AffectsRoute bool   `json:"affectsRoute"`
}

func (blend *weatherBlend) response(start time.Time, duration time.Duration) *weatherBlendResponse {
	if blend == nil {
		return nil
	}
	return &weatherBlendResponse{
		LatestRun:    blend.LatestRun,
		FallbackRun:  blend.FallbackRun,
		Downloaded:   blend.Downloaded,
		Expected:     blend.Expected,
		SwitchAt:     blend.SwitchAt.UTC().Format(time.RFC3339),
		AffectsRoute: start.Add(duration).After(blend.SwitchAt),
	}
}

// blendLatestGRIB remplace les fichiers du run actif par ceux du run le plus
// récent déjà téléchargés, complétés par le run actif au-delà de la dernière
// échéance disponible. Les fichiers d'une autre origine sont laissés tels quels.
func blendLatestGRIB(files []string, status weather.GFSWatcherStatus) ([]string, *weatherBlend) {
	activeRun := runLabel(status.Active.Run)
	latestRun := runLabel(status.Target.Run)
	if !status.HasTarget || activeRun == "" || latestRun <= activeRun || len(status.Target.Files) == 0 {
		return files, nil
	}

	var switchAt time.Time
	for _, path := range status.Target.Files {
		run, valid, ok := gribFileValidity(path)
		if !ok || run != latestRun {
			return files, nil
		}
		if valid.After(switchAt) {
			switchAt = valid
		}
	}

	blended := append([]string(nil), status.Target.Files...)
	for _, path := range files {
		run, valid, ok := gribFileValidity(path)
		if !ok || run != activeRun {
			// Le calcul ne vient pas du run actif du serveur : on n'y touche pas.
			return files, nil
		}
		if valid.After(switchAt) {
			blended = append(blended, path)
		}
	}
	return blended, &weatherBlend{
		LatestRun:   latestRun,
		FallbackRun: activeRun,
		Downloaded:  status.Target.Downloaded,
		Expected:    status.Target.Expected,
		SwitchAt:    switchAt,
	}
}

// gribFileValidity lit le run et l'instant de validité d'un fichier du watcher.
func gribFileValidity(path string) (string, time.Time, bool) {
	match := gribFileName.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return "", time.Time{}, false
	}
	runTime, err := time.Parse("2006010215", match[1])
	if err != nil {
		return "", time.Time{}, false
	}
	hour, err := strconv.Atoi(match[2])
	if err != nil {
		return "", time.Time{}, false
	}
	return match[1], runTime.Add(time.Duration(hour) * time.Hour), true
}

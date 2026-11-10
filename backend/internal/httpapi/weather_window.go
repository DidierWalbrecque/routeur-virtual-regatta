package httpapi

import (
	"sort"
	"time"
)

// weatherWindowMargin prolonge la fenêtre météo au-delà de la durée demandée :
// la dernière couche d'isochrones peut dépasser l'horizon d'un pas de calcul,
// et le pas le plus long proposé reste bien en deçà de six heures.
const weatherWindowMargin = 6 * time.Hour

// gribFilesForWindow ne garde que les échéances utiles à un calcul qui part à
// start et dure duration : la dernière échéance au plus tard au départ, toutes
// celles qui suivent, jusqu'à la première au moins égale à la fin de la
// fenêtre (marge comprise). L'interpolation temporelle dispose ainsi d'une
// échéance de chaque côté de tout instant du calcul.
//
// Si un fichier ne suit pas le nommage du watcher, son échéance est inconnue :
// la liste est alors rendue telle quelle pour ne rien retirer à tort.
func gribFilesForWindow(files []string, start time.Time, duration time.Duration) []string {
	if len(files) == 0 || start.IsZero() || duration < 0 {
		return files
	}

	type dated struct {
		path  string
		valid time.Time
	}
	ordered := make([]dated, 0, len(files))
	for _, path := range files {
		_, valid, ok := gribFileValidity(path)
		if !ok {
			return files
		}
		ordered = append(ordered, dated{path: path, valid: valid})
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].valid.Before(ordered[right].valid)
	})

	end := start.Add(duration + weatherWindowMargin)
	from := 0
	for index, file := range ordered {
		if file.valid.After(start) {
			break
		}
		from = index
	}
	to := len(ordered) - 1
	for index := from; index < len(ordered); index++ {
		if !ordered[index].valid.Before(end) {
			to = index
			break
		}
	}

	selected := make([]string, 0, to-from+1)
	for _, file := range ordered[from : to+1] {
		selected = append(selected, file.path)
	}
	return selected
}

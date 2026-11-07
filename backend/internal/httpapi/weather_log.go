package httpapi

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// gribFileName reconnaît les fichiers du watcher : gfs.<AAAAMMJJHH>.f<hhh>.grb2.
var gribFileName = regexp.MustCompile(`^gfs\.(\d{10})\.f(\d{3})\.grb2$`)

const gribLogTime = "02/01 15:04Z"

// gribUsage résume les fichiers GRIB envoyés à un calcul de route.
type gribUsage struct {
	Count       int
	Runs        []string
	Unknown     []string
	FirstHour   int
	LastHour    int
	FirstValid  time.Time
	LastValid   time.Time
	Missing     []int
	UsedFirst   int
	UsedLast    int
	StartsEarly bool
	EndsLate    time.Duration
}

// describeGRIBFiles analyse les noms de fichiers et la période demandée, sans
// ouvrir les fichiers : runs utilisés, échéances, trous et couverture.
func describeGRIBFiles(paths []string, start time.Time, duration time.Duration) gribUsage {
	usage := gribUsage{Count: len(paths), FirstHour: -1, LastHour: -1, UsedFirst: -1, UsedLast: -1}
	runs := map[string]bool{}
	type forecast struct {
		hour  int
		valid time.Time
	}
	var forecasts []forecast
	for _, path := range paths {
		match := gribFileName.FindStringSubmatch(filepath.Base(path))
		if match == nil {
			usage.Unknown = append(usage.Unknown, filepath.Base(path))
			continue
		}
		runTime, err := time.Parse("2006010215", match[1])
		hour, hourErr := strconv.Atoi(match[2])
		if err != nil || hourErr != nil {
			usage.Unknown = append(usage.Unknown, filepath.Base(path))
			continue
		}
		runs[match[1]] = true
		forecasts = append(forecasts, forecast{hour: hour, valid: runTime.Add(time.Duration(hour) * time.Hour)})
	}
	for run := range runs {
		usage.Runs = append(usage.Runs, run)
	}
	sort.Strings(usage.Runs)
	if len(forecasts) == 0 {
		return usage
	}
	sort.Slice(forecasts, func(left, right int) bool { return forecasts[left].valid.Before(forecasts[right].valid) })

	usage.FirstHour, usage.LastHour = forecasts[0].hour, forecasts[len(forecasts)-1].hour
	usage.FirstValid, usage.LastValid = forecasts[0].valid, forecasts[len(forecasts)-1].valid
	if len(usage.Runs) == 1 {
		present := map[int]bool{}
		for _, item := range forecasts {
			present[item.hour] = true
		}
		for hour := usage.FirstHour; hour <= usage.LastHour; hour += 3 {
			if !present[hour] {
				usage.Missing = append(usage.Missing, hour)
			}
		}
	}

	end := start.Add(duration)
	usage.StartsEarly = start.Before(usage.FirstValid)
	if end.After(usage.LastValid) {
		usage.EndsLate = end.Sub(usage.LastValid)
	}
	// Échéances lues par le calcul : de celle qui précède le départ à celle qui
	// suit la fin, puisque le vent est interpolé entre deux échéances.
	usage.UsedFirst, usage.UsedLast = usage.FirstHour, usage.LastHour
	for _, item := range forecasts {
		if !item.valid.After(start) {
			usage.UsedFirst = item.hour
		}
	}
	for _, item := range forecasts {
		if !item.valid.Before(end) {
			usage.UsedLast = item.hour
			break
		}
	}
	return usage
}

// logGRIBUsage écrit dans le journal les GRIB envoyés au calcul, leur
// chargement et ce qui mérite l'attention.
func logGRIBUsage(paths []string, start time.Time, duration time.Duration, activeRun string, stats cacheLoadStats, milliseconds float64) {
	usage := describeGRIBFiles(paths, start, duration)
	if usage.FirstHour < 0 {
		log.Printf("[GRIB] calcul : %d fichier(s), aucun nom de fichier GFS reconnu (%s)", usage.Count, strings.Join(firstItems(usage.Unknown, 3), ", "))
		return
	}
	log.Printf("[GRIB] calcul : run %s · %d fichier(s) f%03d→f%03d · prévisions du %s au %s",
		strings.Join(usage.Runs, "+"), usage.Count, usage.FirstHour, usage.LastHour,
		usage.FirstValid.Format(gribLogTime), usage.LastValid.Format(gribLogTime))
	log.Printf("[GRIB] période calculée : %s → %s · échéances utilisées f%03d→f%03d",
		start.UTC().Format(gribLogTime), start.Add(duration).UTC().Format(gribLogTime), usage.UsedFirst, usage.UsedLast)
	log.Printf("[GRIB] chargement : %d en cache, %d lu(s) sur disque, %.0f ms", stats.Hits, stats.Misses, milliseconds)

	if len(usage.Runs) > 1 {
		log.Printf("[GRIB] attention : plusieurs runs mélangés dans le calcul (%s)", strings.Join(usage.Runs, ", "))
	}
	if activeRun != "" && (len(usage.Runs) != 1 || usage.Runs[0] != activeRun) {
		log.Printf("[GRIB] attention : le calcul n'utilise pas le run actif du serveur (%s)", activeRun)
	}
	if len(usage.Missing) > 0 {
		shown := make([]string, 0, len(usage.Missing))
		for _, hour := range firstInts(usage.Missing, 10) {
			shown = append(shown, fmt.Sprintf("f%03d", hour))
		}
		log.Printf("[GRIB] attention : %d échéance(s) manquante(s) : %s", len(usage.Missing), strings.Join(shown, ", "))
	}
	if len(usage.Unknown) > 0 {
		log.Printf("[GRIB] attention : %d fichier(s) au nom non reconnu : %s", len(usage.Unknown), strings.Join(firstItems(usage.Unknown, 3), ", "))
	}
	if usage.StartsEarly {
		log.Printf("[GRIB] attention : le départ (%s) précède la première prévision (%s)",
			start.UTC().Format(gribLogTime), usage.FirstValid.Format(gribLogTime))
	}
	if usage.EndsLate > 0 {
		log.Printf("[GRIB] attention : la période demandée dépasse la dernière prévision de %.0f h", usage.EndsLate.Hours())
	}
}

func firstItems(items []string, limit int) []string {
	if len(items) > limit {
		return append(append([]string(nil), items[:limit]...), "…")
	}
	return items
}

func firstInts(items []int, limit int) []int {
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

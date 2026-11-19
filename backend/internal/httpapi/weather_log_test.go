package httpapi

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
)

func gribPaths(run string, hours ...int) []string {
	paths := make([]string, 0, len(hours))
	for _, hour := range hours {
		paths = append(paths, fmt.Sprintf("../data/weather/gfs.%s.f%03d.grb2", run, hour))
	}
	return paths
}

func everyThreeHours(first, last int) []int {
	hours := []int{}
	for hour := first; hour <= last; hour += 3 {
		hours = append(hours, hour)
	}
	return hours
}

func TestDescribeGRIBFilesSummarizesRunAndUsedForecasts(t *testing.T) {
	start := time.Date(2026, 11, 7, 21, 20, 0, 0, time.UTC)
	usage := describeGRIBFiles(gribPaths("2026110712", everyThreeHours(0, 384)...), start, 72*time.Hour)

	if usage.Count != 129 || len(usage.Runs) != 1 || usage.Runs[0] != "2026110712" {
		t.Fatalf("résumé = %+v", usage)
	}
	if usage.FirstHour != 0 || usage.LastHour != 384 || len(usage.Missing) != 0 {
		t.Fatalf("échéances = f%03d→f%03d, manquantes %v", usage.FirstHour, usage.LastHour, usage.Missing)
	}
	// Départ 21:20Z → échéance f009 (21:00Z) ; fin 10/11 21:20Z → échéance f084 (11/11 00:00Z).
	if usage.UsedFirst != 9 || usage.UsedLast != 84 {
		t.Fatalf("échéances utilisées = f%03d→f%03d, attendu f009→f084", usage.UsedFirst, usage.UsedLast)
	}
	if usage.StartsEarly || usage.EndsLate != 0 {
		t.Fatalf("couverture inattendue : %+v", usage)
	}
}

func TestDescribeGRIBFilesReportsGapsMixedRunsAndCoverage(t *testing.T) {
	// Prévisions de 07/11 12:00Z à 08/11 00:00Z ; période de 07/11 10:00Z à 08/11 04:00Z.
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	usage := describeGRIBFiles(gribPaths("2026110712", 0, 3, 9, 12), start, 18*time.Hour)
	if len(usage.Missing) != 1 || usage.Missing[0] != 6 {
		t.Fatalf("manquantes = %v, attendu [6]", usage.Missing)
	}
	if !usage.StartsEarly || usage.EndsLate != 4*time.Hour {
		t.Fatalf("départ trop tôt = %t, dépassement = %v", usage.StartsEarly, usage.EndsLate)
	}

	mixed := describeGRIBFiles(append(gribPaths("2026110706", 0), append(gribPaths("2026110712", 0), "vent.grb2")...), start, time.Hour)
	if len(mixed.Runs) != 2 || len(mixed.Unknown) != 1 {
		t.Fatalf("runs = %v, inconnus = %v", mixed.Runs, mixed.Unknown)
	}
}

func TestLogGRIBUsageWarnsWhenTheActiveRunIsNotUsed(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)

	// Prévisions jusqu'au 09/11 06:00Z ; période jusqu'au 09/11 21:20Z, soit 15 h de trop.
	start := time.Date(2026, 11, 7, 21, 20, 0, 0, time.UTC)
	logGRIBUsage(gribPaths("2026110706", everyThreeHours(0, 48)...), start, 48*time.Hour, nil, "2026110712", cacheLoadStats{Hits: 10, Misses: 7}, 42)

	text := output.String()
	for _, want := range []string{
		"[GRIB] calcul : run 2026110706 · 17 fichier(s) · prévisions du 07/11 06:00Z au 09/11 06:00Z",
		"[GRIB] période calculée : 07/11 21:20Z → 09/11 21:20Z",
		"[GRIB] chargement : 10 en cache, 7 lu(s) sur disque, 42 ms",
		"n'utilise pas le run actif du serveur (2026110712)",
		"dépasse la dernière prévision de 15 h",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("journal sans %q :\n%s", want, text)
		}
	}
}

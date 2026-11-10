package httpapi

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func windowTestFiles(run string, hours ...int) []string {
	files := make([]string, 0, len(hours))
	for _, hour := range hours {
		files = append(files, fmt.Sprintf("../data/weather/gfs.%s.f%03d.grb2", run, hour))
	}
	return files
}

func TestGRIBFilesForWindowKeepsOneStepOnEachSide(t *testing.T) {
	files := windowTestFiles("2026111012", 0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30)
	start := time.Date(2026, 11, 10, 16, 30, 0, 0, time.UTC) // entre f003 et f006
	got := gribFilesForWindow(files, start, 2*time.Hour)     // fin + 6 h = 00:30 → f015

	want := windowTestFiles("2026111012", 3, 6, 9, 12, 15)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gribFilesForWindow() = %v, attendu %v", got, want)
	}
}

func TestGRIBFilesForWindowStartsOnExactStep(t *testing.T) {
	files := windowTestFiles("2026111012", 0, 3, 6, 9, 12, 15)
	start := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC) // exactement f003
	got := gribFilesForWindow(files, start, 0)              // fin = 21:00 → f009

	want := windowTestFiles("2026111012", 3, 6, 9)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gribFilesForWindow() = %v, attendu %v", got, want)
	}
}

func TestGRIBFilesForWindowSortsBlendedRuns(t *testing.T) {
	latest := windowTestFiles("2026111018", 0, 3, 6)
	fallback := windowTestFiles("2026111012", 0, 3, 15, 18, 21) // f015 = 11/11 03:00
	files := append(append([]string{}, latest...), fallback...)
	start := time.Date(2026, 11, 10, 19, 0, 0, 0, time.UTC)
	got := gribFilesForWindow(files, start, 4*time.Hour) // fin + 6 h = 05:00 → f018 du run 12

	want := []string{latest[0], latest[1], latest[2], fallback[2], fallback[3]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gribFilesForWindow() = %v, attendu %v", got, want)
	}
}

func TestGRIBFilesForWindowKeepsBoundsOutsideCoverage(t *testing.T) {
	files := windowTestFiles("2026111012", 0, 3, 6)
	before := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	if got := gribFilesForWindow(files, before, time.Hour); !reflect.DeepEqual(got, files) {
		t.Fatalf("départ avant le run : %v, attendu %v", got, files)
	}
	after := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	if got := gribFilesForWindow(files, after, time.Hour); !reflect.DeepEqual(got, files[2:]) {
		t.Fatalf("départ après le run : %v, attendu %v", got, files[2:])
	}
}

func TestGRIBFilesForWindowLeavesUnknownNamesUntouched(t *testing.T) {
	files := []string{"../data/weather/gfs.2026111012.f000.grb2", "custom/forecast.grb2"}
	start := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if got := gribFilesForWindow(files, start, time.Hour); !reflect.DeepEqual(got, files) {
		t.Fatalf("gribFilesForWindow() = %v, attendu la liste inchangée", got)
	}
}

func TestGRIBFilesForWindowFullRunFitsTheRequestedHorizon(t *testing.T) {
	hours := make([]int, 0, 129)
	for hour := 0; hour <= 384; hour += 3 {
		hours = append(hours, hour)
	}
	files := windowTestFiles("2026111012", hours...)
	start := time.Date(2026, 11, 10, 13, 0, 0, 0, time.UTC)
	got := gribFilesForWindow(files, start, 48*time.Hour)
	// 12:00 (avant le départ) jusqu'à la première échéance ≥ 13:00 + 54 h.
	if len(got) != 20 {
		t.Fatalf("%d échéances gardées sur %d, attendu 20", len(got), len(files))
	}
}

func TestLoadCachedWeatherProviderEvictsGridsOutsideTheRequest(t *testing.T) {
	routeDataCache.Lock()
	saved := routeDataCache.grids
	routeDataCache.grids = map[string]cachedGrid{
		"ancien-run.grb2": {},
		"autre.grb2":      {},
	}
	routeDataCache.Unlock()
	t.Cleanup(func() {
		routeDataCache.Lock()
		routeDataCache.grids = saved
		routeDataCache.Unlock()
	})

	_, _, err := loadCachedWeatherProvider("fichier-absent.grb2")
	if err == nil {
		t.Fatal("loadCachedWeatherProvider() devait échouer sur un fichier absent")
	}

	routeDataCache.Lock()
	defer routeDataCache.Unlock()
	if len(routeDataCache.grids) != 0 {
		t.Fatalf("cache = %v, attendu vidé des échéances hors du calcul", routeDataCache.grids)
	}
}

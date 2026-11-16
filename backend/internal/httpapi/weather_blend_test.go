package httpapi

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func gfsRun(t *testing.T, date string, cycle int) weather.GFSForecastRequest {
	t.Helper()
	run, err := weather.NewGFSForecastRequest(date, cycle, 0)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func watcherStatus(t *testing.T, latestHours []int) weather.GFSWatcherStatus {
	t.Helper()
	return weather.GFSWatcherStatus{
		Active:    weather.GFSRefreshResult{Run: gfsRun(t, "20261107", 12), Files: gribPaths("2026110712", everyThreeHours(0, 384)...)},
		HasTarget: true,
		Target: weather.GFSRunProgress{
			Run:        gfsRun(t, "20261107", 18),
			Downloaded: len(latestHours),
			Expected:   129,
			Files:      gribPaths("2026110718", latestHours...),
		},
	}
}

func TestBlendLatestGRIBUsesTheNewestRunThenThePreviousOne(t *testing.T) {
	status := watcherStatus(t, everyThreeHours(0, 12))
	files, blend := blendLatestGRIB(status.Active.Files, status)
	if blend == nil {
		t.Fatal("le run 18Z partiel devrait être utilisé")
	}
	// 18Z f000→f012 couvre jusqu'au 08/11 06:00Z ; le 12Z prend le relais à f021 (08/11 09:00Z).
	if blend.LatestRun != "2026110718" || blend.FallbackRun != "2026110712" || blend.Downloaded != 5 || blend.Expected != 129 {
		t.Fatalf("bascule = %+v", blend)
	}
	if want := time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC); !blend.SwitchAt.Equal(want) {
		t.Fatalf("bascule à %v, attendu %v", blend.SwitchAt, want)
	}
	// 12Z f000→f003 (avant 18:00Z, départ dans le passé), 18Z f000→f012, puis 12Z f021→f384.
	if len(files) != 2+5+(129-7) {
		t.Fatalf("%d fichiers, attendu %d", len(files), 2+5+(129-7))
	}
	if filepath.Base(files[1]) != "gfs.2026110712.f003.grb2" || filepath.Base(files[2]) != "gfs.2026110718.f000.grb2" {
		t.Fatalf("début = %s puis %s", filepath.Base(files[1]), filepath.Base(files[2]))
	}
	if filepath.Base(files[6]) != "gfs.2026110718.f012.grb2" || filepath.Base(files[7]) != "gfs.2026110712.f021.grb2" {
		t.Fatalf("raccord = %s puis %s", filepath.Base(files[6]), filepath.Base(files[7]))
	}
	seen := map[time.Time]bool{}
	for _, path := range files {
		_, valid, ok := gribFileValidity(path)
		if !ok || seen[valid] {
			t.Fatalf("échéance en double ou illisible : %s", path)
		}
		seen[valid] = true
	}
}

func TestBlendLatestGRIBSaysWhetherTheRouteGoesPastTheNewestRun(t *testing.T) {
	_, blend := blendLatestGRIB(watcherStatus(t, everyThreeHours(0, 12)).Active.Files, watcherStatus(t, everyThreeHours(0, 12)))
	start := time.Date(2026, 11, 7, 23, 0, 0, 0, time.UTC)
	if blend.response(start, 4*time.Hour).AffectsRoute {
		t.Fatal("une route de 4 h finit avant la bascule : aucun impact attendu")
	}
	if !blend.response(start, 24*time.Hour).AffectsRoute {
		t.Fatal("une route de 24 h dépasse la bascule : impact attendu")
	}
	var missing *weatherBlend
	if missing.response(start, time.Hour) != nil {
		t.Fatal("sans bascule, aucune information ne doit être envoyée")
	}
}

func TestBlendLatestGRIBLeavesOtherCasesUntouched(t *testing.T) {
	status := watcherStatus(t, everyThreeHours(0, 12))

	noTarget := status
	noTarget.HasTarget = false
	if _, blend := blendLatestGRIB(status.Active.Files, noTarget); blend != nil {
		t.Error("sans run en téléchargement, rien ne doit changer")
	}

	nothingYet := watcherStatus(t, nil)
	if _, blend := blendLatestGRIB(status.Active.Files, nothingYet); blend != nil {
		t.Error("sans échéance téléchargée, le run actif doit rester seul")
	}

	older := status
	older.Target.Run = gfsRun(t, "20261107", 6)
	older.Target.Files = gribPaths("2026110706", 0, 3)
	if _, blend := blendLatestGRIB(status.Active.Files, older); blend != nil {
		t.Error("un run cible plus ancien ne doit jamais remplacer le run actif")
	}

	manual := gribPaths("2026110706", everyThreeHours(0, 48)...)
	files, blend := blendLatestGRIB(manual, status)
	if blend != nil || len(files) != len(manual) {
		t.Error("des fichiers choisis à la main ne doivent pas être modifiés")
	}
}

func TestLogGRIBUsageAnnouncesTheSwitchToTheOlderRun(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)

	status := watcherStatus(t, everyThreeHours(0, 12))
	files, blend := blendLatestGRIB(status.Active.Files, status)
	start := time.Date(2026, 11, 7, 23, 0, 0, 0, time.UTC)
	logGRIBUsage(files, start, 24*time.Hour, blend, "2026110712", cacheLoadStats{}, 0)

	text := output.String()
	for _, want := range []string{
		"[GRIB] calcul : run 2026110712+2026110718",
		"run le plus récent 2026110718 en téléchargement (5/129 échéances) : utilisé jusqu'au 08/11 06:00Z, puis run 2026110712",
		"attention : la route dépasse le 08/11 06:00Z",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("journal sans %q :\n%s", want, text)
		}
	}
	if strings.Contains(text, "plusieurs runs mélangés") || strings.Contains(text, "n'utilise pas le run actif") {
		t.Errorf("avertissements inutiles pour une bascule voulue :\n%s", text)
	}
}

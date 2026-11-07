package weather

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// progressiveDownloader simule NOAA : pour chaque cycle, les échéances jusqu'à
// published[cycle] sont disponibles, les suivantes ne sont pas encore publiées.
type progressiveDownloader struct {
	published map[int]int
}

func (downloader *progressiveDownloader) Download(_ context.Context, request GFSForecastRequest, directory string) (string, error) {
	last, ok := downloader.published[request.Cycle]
	if !ok || request.ForecastHour > last {
		return "", errors.New("pas encore publié")
	}
	path := filepath.Join(directory, request.LocalFileName())
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, []byte("grib"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func TestGFSWatcherSwitchesOnlyWhenTargetRunIsComplete(t *testing.T) {
	directory := t.TempDir()
	hours := []int{0, 3, 6}
	downloader := &progressiveDownloader{published: map[int]int{6: 6, 12: 3}}
	now := time.Date(2026, 11, 7, 11, 15, 0, 0, time.UTC)
	watcher := NewGFSWatcher(downloader, directory, hours, func() time.Time { return now })

	status, _, err := watcher.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Active.Run.Cycle != 6 || len(status.Active.Files) != len(hours) {
		t.Fatalf("run actif attendu 06Z complet, obtenu %+v", status.Active)
	}

	now = time.Date(2026, 11, 7, 12, 15, 0, 0, time.UTC)
	status, removed, err := watcher.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Active.Run.Cycle != 6 {
		t.Fatalf("bascule prématurée vers le run %02dZ", status.Active.Run.Cycle)
	}
	if status.Target.Downloaded != 2 || status.Target.Complete() || status.Waiting == "" {
		t.Fatalf("progression 12Z inattendue: %+v, attente=%q", status.Target, status.Waiting)
	}
	if len(removed) != 0 {
		t.Fatalf("aucun fichier ne doit être supprimé pendant la publication, supprimés=%v", removed)
	}

	downloader.published[12] = 6
	status, removed, err = watcher.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Active.Run.Cycle != 12 || len(status.Active.Files) != len(hours) {
		t.Fatalf("run actif attendu 12Z complet, obtenu %+v", status.Active)
	}
	if len(removed) != len(hours) {
		t.Fatalf("anciens fichiers supprimés = %d, attendu %d", len(removed), len(hours))
	}
}

func TestCompleteGFSRunSkipsPartialNewestRun(t *testing.T) {
	directory := t.TempDir()
	hours := []int{0, 3, 6}
	for _, name := range []string{
		"gfs.2026110706.f000.grb2", "gfs.2026110706.f003.grb2", "gfs.2026110706.f006.grb2",
		"gfs.2026110712.f000.grb2", "gfs.2026110712.f003.grb2",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("grib"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := CompleteGFSRun(directory, hours)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Cycle != 6 || len(result.Files) != len(hours) {
		t.Fatalf("run complet attendu 06Z, obtenu %+v", result.Run)
	}

	// Le run partiel 12Z doit survivre au nettoyage déclenché par le run 06Z.
	run06, err := NewGFSForecastRequest("20261107", 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveOldGFSFiles(directory, run06); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gfs.2026110712.f000.grb2", "gfs.2026110712.f003.grb2"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("fichier du run plus récent supprimé: %v", err)
		}
	}
}

func TestCompleteGFSRunEmptyWhenNoRunIsComplete(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "gfs.2026110712.f000.grb2"), []byte("grib"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CompleteGFSRun(directory, []int{0, 3})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Cycle != 0 || len(result.Files) != 0 {
		t.Fatalf("aucun run complet attendu, obtenu %+v", result)
	}
}

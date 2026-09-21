package weather

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type refreshDownloader struct {
	availableCycle int
	requests       []GFSForecastRequest
}

func (downloader *refreshDownloader) Download(_ context.Context, request GFSForecastRequest, _ string) (string, error) {
	downloader.requests = append(downloader.requests, request)
	if request.Cycle != downloader.availableCycle {
		return "", errors.New("run indisponible")
	}
	return request.LocalFileName(), nil
}

func TestRefreshLatestGFSFallsBackToPreviousRun(t *testing.T) {
	downloader := &refreshDownloader{availableCycle: 0}
	result, err := RefreshLatestGFS(context.Background(), downloader, GFSRefreshOptions{
		ForecastHours: []int{6, 0}, OutputDirectory: "weather", LookbackHours: 12,
		Now: time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Cycle != 0 || len(result.Files) != 2 {
		t.Fatalf("résultat inattendu: %+v", result)
	}
	if len(downloader.requests) != 3 {
		t.Fatalf("nombre de tentatives = %d, attendu 3", len(downloader.requests))
	}
}

func TestRefreshLatestGFSRejectsInvalidHour(t *testing.T) {
	_, err := RefreshLatestGFS(context.Background(), &refreshDownloader{}, GFSRefreshOptions{
		ForecastHours: []int{385}, OutputDirectory: "weather", LookbackHours: 12,
	})
	if err == nil {
		t.Fatal("une échéance hors limites devait être refusée")
	}
}

func TestRemoveOldGFSFilesKeepsCurrentAndUnknownFiles(t *testing.T) {
	directory := t.TempDir()
	files := []string{
		"gfs.2026092006.f000.grb2",
		"gfs.2026092012.f024.grb2",
		"gfs.2026092006.f024.grb2",
		"notes.txt",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	current, err := NewGFSForecastRequest("20260920", 12, 0)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := RemoveOldGFSFiles(directory, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("fichiers supprimés = %d, attendu 2", len(removed))
	}
	for _, name := range []string{"gfs.2026092012.f024.grb2", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("fichier %q absent après nettoyage: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "gfs.2026092006.f000.grb2")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ancien fichier encore présent ou erreur inattendue: %v", err)
	}
}

func TestRefreshLatestGFSDoesNotCleanAfterFailedRun(t *testing.T) {
	directory := t.TempDir()
	oldPath := filepath.Join(directory, "gfs.2026092006.f000.grb2")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := RefreshLatestGFS(context.Background(), &refreshDownloader{availableCycle: 99}, GFSRefreshOptions{
		ForecastHours: []int{0}, OutputDirectory: directory, LookbackHours: 6,
		Now: time.Date(2026, 9, 20, 12, 15, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("un téléchargement échoué devait retourner une erreur")
	}
	if _, statErr := os.Stat(oldPath); statErr != nil {
		t.Fatalf("ancien fichier supprimé après échec: %v", statErr)
	}
}

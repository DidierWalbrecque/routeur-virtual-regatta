package weather

import (
	"context"
	"errors"
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

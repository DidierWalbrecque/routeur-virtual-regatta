package land

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const naturalEarthTestDocument = `{
	"type":"FeatureCollection",
	"features":[{
		"type":"Feature",
		"geometry":{
			"type":"MultiPolygon",
			"coordinates":[
				[[[0,0],[1,0],[1,1],[0,1],[0,0]]],
				[[[2,0],[3,0],[3,1],[2,1],[2,0]]]
			]
		}
	}]
}`

func TestNaturalEarthDownloaderDownloadsAndReusesValidatedFile(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			requestCount.Add(1)
			if request.Header.Get("User-Agent") == "" {
				t.Error("User-Agent absent")
			}
			response.Header().Set("Content-Type", "application/geo+json")
			_, _ = response.Write([]byte(naturalEarthTestDocument))
		},
	))
	defer server.Close()

	downloader := NewNaturalEarthDownloader()
	downloader.HTTPClient = server.Client()
	downloader.SourceURL = server.URL
	outputDirectory := t.TempDir()

	result, err := downloader.Download(context.Background(), outputDirectory)
	if err != nil {
		t.Fatalf("téléchargement refusé: %v", err)
	}
	if result.Reused {
		t.Fatal("premier téléchargement marqué comme réutilisé")
	}
	if result.PolygonCount != 2 {
		t.Fatalf("polygones = %d, attendus 2", result.PolygonCount)
	}
	if result.Size != int64(len(naturalEarthTestDocument)) {
		t.Fatalf(
			"taille = %d, attendue %d",
			result.Size,
			len(naturalEarthTestDocument),
		)
	}
	expectedPath := filepath.Join(
		outputDirectory,
		NaturalEarthLandFileName,
	)
	if result.Path != expectedPath {
		t.Fatalf("chemin = %q, attendu %q", result.Path, expectedPath)
	}

	reused, err := downloader.Download(context.Background(), outputDirectory)
	if err != nil {
		t.Fatalf("réutilisation refusée: %v", err)
	}
	if !reused.Reused {
		t.Fatal("fichier existant non marqué comme réutilisé")
	}
	if requestCount.Load() != 1 {
		t.Fatalf("requêtes HTTP = %d, attendue 1", requestCount.Load())
	}
}

func TestNaturalEarthDownloaderRejectsInvalidDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			_, _ = response.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
		},
	))
	defer server.Close()

	downloader := NewNaturalEarthDownloader()
	downloader.HTTPClient = server.Client()
	downloader.SourceURL = server.URL
	outputDirectory := t.TempDir()

	_, err := downloader.Download(context.Background(), outputDirectory)
	if err == nil || !strings.Contains(err.Error(), "téléchargé invalide") {
		t.Fatalf("erreur = %v, téléchargement invalide attendu", err)
	}
	destination := filepath.Join(
		outputDirectory,
		NaturalEarthLandFileName,
	)
	if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination invalide installée: %v", statErr)
	}
}

func TestNaturalEarthDownloaderRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			http.Error(response, "indisponible", http.StatusServiceUnavailable)
		},
	))
	defer server.Close()

	downloader := NewNaturalEarthDownloader()
	downloader.HTTPClient = server.Client()
	downloader.SourceURL = server.URL

	_, err := downloader.Download(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("erreur = %v, statut HTTP attendu", err)
	}
}

func TestNaturalEarthDownloaderEnforcesSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			_, _ = response.Write([]byte(naturalEarthTestDocument))
		},
	))
	defer server.Close()

	downloader := NewNaturalEarthDownloader()
	downloader.HTTPClient = server.Client()
	downloader.SourceURL = server.URL
	downloader.MaxDownloadSize = 16

	_, err := downloader.Download(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("erreur = %v, limite attendue", err)
	}
}

func TestNaturalEarthDownloaderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewNaturalEarthDownloader().Download(ctx, t.TempDir())
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("erreur = %v, annulation attendue", err)
	}
}

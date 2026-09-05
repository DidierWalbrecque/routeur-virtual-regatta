package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewGFSForecastRequest(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 3)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}

	wantDate := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if !request.Date.Equal(wantDate) {
		t.Fatalf("date = %s, attendue %s", request.Date, wantDate)
	}
	if request.SourceFileName() != "gfs.t06z.pgrb2.0p25.f003" {
		t.Fatalf("nom NOAA = %q", request.SourceFileName())
	}
	if request.LocalFileName() != "gfs.2026090506.f003.grb2" {
		t.Fatalf("nom local = %q", request.LocalFileName())
	}
	wantValidAt := time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC)
	if !request.ValidAt().Equal(wantValidAt) {
		t.Fatalf(
			"échéance = %s, attendue %s",
			request.ValidAt(),
			wantValidAt,
		)
	}
}

func TestGFSForecastRequestRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name         string
		date         string
		cycle        int
		forecastHour int
		wantErr      string
	}{
		{
			name:    "date invalide",
			date:    "05-09-2026",
			wantErr: "format YYYYMMDD",
		},
		{
			name:    "cycle invalide",
			date:    "20260905",
			cycle:   5,
			wantErr: "cycle GFS",
		},
		{
			name:         "échéance négative",
			date:         "20260905",
			forecastHour: -1,
			wantErr:      "hors limites",
		},
		{
			name:         "échéance trop grande",
			date:         "20260905",
			forecastHour: 385,
			wantErr:      "hors limites",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGFSForecastRequest(
				test.date,
				test.cycle,
				test.forecastHour,
			)
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf(
					"erreur = %q, doit contenir %q",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestNOAAGFSDownloaderForecastURL(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 3)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}
	downloader := NewNOAAGFSDownloader()

	rawURL, err := downloader.forecastURL(request)
	if err != nil {
		t.Fatalf("URL refusée: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("URL produite invalide: %v", err)
	}

	if parsed.Scheme != "https" ||
		parsed.Host != "nomads.ncep.noaa.gov" ||
		parsed.Path != "/cgi-bin/filter_gfs_0p25.pl" {
		t.Fatalf("URL de base inattendue: %s", parsed)
	}

	query := parsed.Query()
	expected := map[string]string{
		"file":                  "gfs.t06z.pgrb2.0p25.f003",
		"lev_10_m_above_ground": "on",
		"var_UGRD":              "on",
		"var_VGRD":              "on",
		"leftlon":               "0",
		"rightlon":              "360",
		"toplat":                "90",
		"bottomlat":             "-90",
		"dir":                   "/gfs.20260905/06/atmos",
	}
	for key, want := range expected {
		if got := query.Get(key); got != want {
			t.Fatalf("%s = %q, attendu %q", key, got, want)
		}
	}
}

func TestNOAAGFSDownloaderDownload(t *testing.T) {
	const body = "GRIB synthetic test data 7777"
	request, err := NewGFSForecastRequest("20260905", 6, 3)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, httpRequest *http.Request) {
			if got := httpRequest.URL.Query().Get("var_UGRD"); got != "on" {
				t.Errorf("var_UGRD = %q", got)
			}
			if got := httpRequest.URL.Query().Get("var_VGRD"); got != "on" {
				t.Errorf("var_VGRD = %q", got)
			}
			_, _ = writer.Write([]byte(body))
		},
	))
	defer server.Close()

	downloader := NewNOAAGFSDownloader()
	downloader.Endpoint = server.URL
	downloader.HTTPClient = server.Client()
	downloader.validateFile = func(
		path string,
		gotRequest GFSForecastRequest,
	) error {
		if gotRequest != request {
			t.Fatalf("demande de validation = %+v", gotRequest)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(content) != body {
			t.Fatalf("contenu = %q", content)
		}
		return nil
	}

	outputDirectory := t.TempDir()
	path, err := downloader.Download(
		context.Background(),
		request,
		outputDirectory,
	)
	if err != nil {
		t.Fatalf("téléchargement refusé: %v", err)
	}

	wantPath := filepath.Join(outputDirectory, request.LocalFileName())
	if path != wantPath {
		t.Fatalf("chemin = %q, attendu %q", path, wantPath)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture du résultat impossible: %v", err)
	}
	if string(content) != body {
		t.Fatalf("contenu final = %q", content)
	}
}

func TestNOAAGFSDownloaderReusesValidExistingFile(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 0)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}
	outputDirectory := t.TempDir()
	path := filepath.Join(outputDirectory, request.LocalFileName())
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("préparation impossible: %v", err)
	}

	validations := 0
	downloader := NewNOAAGFSDownloader()
	downloader.HTTPClient = nil
	downloader.validateFile = func(
		gotPath string,
		gotRequest GFSForecastRequest,
	) error {
		validations++
		if gotPath != path || gotRequest != request {
			t.Fatalf("validation de %q avec %+v", gotPath, gotRequest)
		}
		return nil
	}

	got, err := downloader.Download(
		context.Background(),
		request,
		outputDirectory,
	)
	if err != nil {
		t.Fatalf("réutilisation refusée: %v", err)
	}
	if got != path {
		t.Fatalf("chemin = %q, attendu %q", got, path)
	}
	if validations != 1 {
		t.Fatalf("validations = %d, attendu 1", validations)
	}
}

func TestNOAAGFSDownloaderRejectsHTTPError(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 0)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "run not ready", http.StatusNotFound)
		},
	))
	defer server.Close()

	downloader := NewNOAAGFSDownloader()
	downloader.Endpoint = server.URL
	downloader.HTTPClient = server.Client()
	downloader.validateFile = func(
		string,
		GFSForecastRequest,
	) error {
		t.Fatal("validation appelée après une erreur HTTP")
		return nil
	}

	_, err = downloader.Download(
		context.Background(),
		request,
		t.TempDir(),
	)
	if err == nil {
		t.Fatal("erreur attendue, aucune reçue")
	}
	if !strings.Contains(err.Error(), "404") ||
		!strings.Contains(err.Error(), "run not ready") {
		t.Fatalf("erreur inattendue: %v", err)
	}
}

func TestNOAAGFSDownloaderRejectsOversizedResponse(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 0)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte("12345"))
		},
	))
	defer server.Close()

	downloader := NewNOAAGFSDownloader()
	downloader.Endpoint = server.URL
	downloader.HTTPClient = server.Client()
	downloader.MaxDownloadSize = 4
	downloader.validateFile = func(
		string,
		GFSForecastRequest,
	) error {
		t.Fatal("validation appelée pour un fichier trop grand")
		return nil
	}

	_, err = downloader.Download(
		context.Background(),
		request,
		t.TempDir(),
	)
	if err == nil || !strings.Contains(err.Error(), "supérieur à la limite") {
		t.Fatalf("erreur = %v", err)
	}
}

func TestNOAAGFSDownloaderRejectsCancelledContext(t *testing.T) {
	request, err := NewGFSForecastRequest("20260905", 6, 0)
	if err != nil {
		t.Fatalf("demande refusée: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = NewNOAAGFSDownloader().Download(
		ctx,
		request,
		t.TempDir(),
	)
	if err == nil || !strings.Contains(err.Error(), "annulé") {
		t.Fatalf("erreur = %v", err)
	}
}

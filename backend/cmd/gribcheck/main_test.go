package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseOptionsAcceptsSinglePositionalFile(t *testing.T) {
	options, err := parseOptions([]string{"forecast.grb2"})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if len(options.files) != 1 || options.files[0] != "forecast.grb2" {
		t.Fatalf("fichiers = %v", options.files)
	}
	if options.hasPosition {
		t.Fatal("position détectée sans coordonnées")
	}
	if !options.instant.IsZero() {
		t.Fatalf("instant inattendu: %s", options.instant)
	}
}

func TestParseOptionsAcceptsForecastSeriesQuery(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f000.grb2",
		"-file", "f003.grb2",
		"-latitude", "13.121111",
		"-longitude", "121.586667",
		"-at", "2026-04-29T07:30:00Z",
	})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if len(options.files) != 2 {
		t.Fatalf("nombre de fichiers = %d, attendu 2", len(options.files))
	}
	if options.files[0] != "f000.grb2" ||
		options.files[1] != "f003.grb2" {
		t.Fatalf("fichiers = %v", options.files)
	}
	if !options.hasPosition {
		t.Fatal("position non détectée")
	}
	if options.latitude != 13.121111 ||
		options.longitude != 121.586667 {
		t.Fatalf(
			"position = %.6f, %.6f",
			options.latitude,
			options.longitude,
		)
	}
	wantAt := time.Date(2026, 4, 29, 7, 30, 0, 0, time.UTC)
	if !options.instant.Equal(wantAt) {
		t.Fatalf("instant = %s, attendu %s", options.instant, wantAt)
	}
}

func TestParseOptionsAllowsSingleFileQueryWithoutInstant(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f003.grb2",
		"-latitude", "13",
		"-longitude", "121",
	})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if !options.hasPosition {
		t.Fatal("position non détectée")
	}
	if !options.instant.IsZero() {
		t.Fatalf("instant inattendu: %s", options.instant)
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "aucun fichier",
			wantErr: "au moins un chemin",
		},
		{
			name: "fichier vide",
			args: []string{
				"-file", "",
			},
			wantErr: "arguments invalides",
		},
		{
			name: "fichier nommé et positionnel",
			args: []string{
				"-file", "f000.grb2",
				"f003.grb2",
			},
			wantErr: "arguments inattendus",
		},
		{
			name: "latitude seule",
			args: []string{
				"-file", "f000.grb2",
				"-latitude", "13",
			},
			wantErr: "doivent être fournis ensemble",
		},
		{
			name: "instant sans position",
			args: []string{
				"-file", "f000.grb2",
				"-at", "2026-04-29T06:00:00Z",
			},
			wantErr: "nécessite également",
		},
		{
			name: "instant invalide",
			args: []string{
				"-file", "f000.grb2",
				"-latitude", "13",
				"-longitude", "121",
				"-at", "29/04/2026 06:00",
			},
			wantErr: "format RFC3339 attendu",
		},
		{
			name: "série sans instant",
			args: []string{
				"-file", "f000.grb2",
				"-file", "f003.grb2",
				"-latitude", "13",
				"-longitude", "121",
			},
			wantErr: "-at est requis",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions(test.args)
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

func TestFilePathsString(t *testing.T) {
	paths := filePaths{"f000.grb2", "f003.grb2"}
	if got := paths.String(); got != "f000.grb2,f003.grb2" {
		t.Fatalf("String() = %q", got)
	}
}

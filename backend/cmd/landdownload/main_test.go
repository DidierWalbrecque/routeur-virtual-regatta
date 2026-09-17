package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
)

type fakeNaturalEarthDownloader struct {
	result          land.NaturalEarthDownload
	err             error
	outputDirectory string
}

func (downloader *fakeNaturalEarthDownloader) Download(
	_ context.Context,
	outputDirectory string,
) (land.NaturalEarthDownload, error) {
	downloader.outputDirectory = outputDirectory
	return downloader.result, downloader.err
}

func TestParseOptionsUsesDefaultOutputDirectory(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if opts.outputDirectory != "../data/land" {
		t.Fatalf(
			"dossier = %q, attendu ../data/land",
			opts.outputDirectory,
		)
	}
}

func TestParseOptionsAcceptsExplicitOutputDirectory(t *testing.T) {
	opts, err := parseOptions([]string{"-output", "  cache/land  "})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if opts.outputDirectory != "cache/land" {
		t.Fatalf(
			"dossier = %q, attendu cache/land",
			opts.outputDirectory,
		)
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "dossier vide",
			args: []string{"-output", "   "},
		},
		{
			name: "argument positionnel",
			args: []string{"unexpected"},
		},
		{
			name: "option inconnue",
			args: []string{"-unknown"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseOptions(test.args); err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
		})
	}
}

func TestRunWithDownloaderReportsDownloadedFile(t *testing.T) {
	downloader := &fakeNaturalEarthDownloader{
		result: land.NaturalEarthDownload{
			Path:         "../data/land/natural-earth-10m-land.geojson",
			Size:         1234,
			PolygonCount: 42,
		},
	}
	var output strings.Builder

	err := runWithDownloader(
		[]string{"-output", "../data/custom-land"},
		&output,
		downloader,
	)
	if err != nil {
		t.Fatalf("commande refusée: %v", err)
	}
	if downloader.outputDirectory != "../data/custom-land" {
		t.Fatalf(
			"dossier transmis = %q",
			downloader.outputDirectory,
		)
	}
	for _, expected := range []string{
		"Téléchargement et validation",
		"Téléchargé :",
		"1234 octets",
		"42 polygones",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf(
				"sortie %q sans %q",
				output.String(),
				expected,
			)
		}
	}
}

func TestRunWithDownloaderReportsReusedFile(t *testing.T) {
	downloader := &fakeNaturalEarthDownloader{
		result: land.NaturalEarthDownload{
			Path:         "land.geojson",
			Size:         100,
			PolygonCount: 2,
			Reused:       true,
		},
	}
	var output strings.Builder

	if err := runWithDownloader(nil, &output, downloader); err != nil {
		t.Fatalf("commande refusée: %v", err)
	}
	if !strings.Contains(output.String(), "Réutilisé :") {
		t.Fatalf("sortie = %q, réutilisation absente", output.String())
	}
}

func TestRunWithDownloaderPropagatesDownloadError(t *testing.T) {
	downloader := &fakeNaturalEarthDownloader{
		err: errors.New("réseau indisponible"),
	}

	err := runWithDownloader(nil, ioDiscard{}, downloader)
	if err == nil || !strings.Contains(err.Error(), "réseau indisponible") {
		t.Fatalf("erreur = %v, erreur réseau attendue", err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(value []byte) (int, error) {
	return len(value), nil
}

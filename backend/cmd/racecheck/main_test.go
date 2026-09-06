package main

import (
	"io"
	"math"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestParseOptionsAcceptsVirtualRegattaPosition(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f000.grb2",
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-position", "13°N 07'16\" - 121°E 35'12\"",
		"-at", "2026-09-05T07:30:00Z",
		"-heading", "321",
		"-minutes", "10",
		"-owned", "code0,foils",
	}, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if len(options.files) != 2 {
		t.Fatalf("fichiers = %v", options.files)
	}
	if options.polarPath != "boat.json" {
		t.Fatalf("polaire = %q", options.polarPath)
	}
	assertClose(
		t,
		"latitude",
		float64(options.position.Latitude),
		13.12111111111111,
	)
	assertClose(
		t,
		"longitude",
		float64(options.position.Longitude),
		121.58666666666667,
	)
	if options.heading != 321 {
		t.Fatalf("cap = %v", options.heading)
	}
	if options.minutes != 10 {
		t.Fatalf("durée = %v", options.minutes)
	}
	if options.stepMinutes != 10 {
		t.Fatalf("durée d'un pas = %v, attendue 10", options.stepMinutes)
	}
	if !options.owned.Has(equipment.Code0) ||
		!options.owned.Has(equipment.Foils) {
		t.Fatalf("équipements = %v", options.owned)
	}
}

func TestParseOptionsAcceptsDecimalPosition(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-latitude", "13.121111",
		"-longitude", "121.586667",
		"-at", "2026-09-05T09:00:00Z",
		"-heading", "90",
		"-minutes", "5",
	}, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	assertClose(
		t,
		"latitude",
		float64(options.position.Latitude),
		13.121111,
	)
	assertClose(
		t,
		"longitude",
		float64(options.position.Longitude),
		121.586667,
	)
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	base := []string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-latitude", "13",
		"-longitude", "121",
		"-at", "2026-09-05T09:00:00Z",
		"-heading", "90",
		"-minutes", "5",
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "fichier absent",
			args:    base[2:],
			wantErr: "-file",
		},
		{
			name: "polaire absente",
			args: append(
				[]string{"-file", "f003.grb2"},
				base[4:]...,
			),
			wantErr: "-polar",
		},
		{
			name: "position absente",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-at", "2026-09-05T09:00:00Z",
				"-heading", "90",
				"-minutes", "5",
			},
			wantErr: "fournir -position",
		},
		{
			name: "positions mélangées",
			args: append(
				append([]string{}, base...),
				"-position", "13°N 00'00\" - 121°E 00'00\"",
			),
			wantErr: "ne peut pas être combiné",
		},
		{
			name: "instant absent",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-latitude", "13",
				"-longitude", "121",
				"-heading", "90",
				"-minutes", "5",
			},
			wantErr: "-at",
		},
		{
			name: "cap absent",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-latitude", "13",
				"-longitude", "121",
				"-at", "2026-09-05T09:00:00Z",
				"-minutes", "5",
			},
			wantErr: "-heading",
		},
		{
			name: "durée nulle",
			args: append(
				append([]string{}, base[:len(base)-1]...),
				"0",
			),
			wantErr: "-minutes",
		},
		{
			name: "équipement inconnu",
			args: append(
				append([]string{}, base...),
				"-owned", "radio",
			),
			wantErr: "équipements possédés",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions(test.args, io.Discard)
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

func TestFormatTack(t *testing.T) {
	tests := map[navigation.Tack]string{
		navigation.TackPort:      "bâbord",
		navigation.TackStarboard: "tribord",
		navigation.TackUndefined: "indéfinie",
	}
	for tack, want := range tests {
		if got := formatTack(tack); got != want {
			t.Errorf("formatTack(%q) = %q, attendu %q", tack, got, want)
		}
	}
}

func assertClose(
	t *testing.T,
	name string,
	got,
	want float64,
) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.12f, attendu %.12f", name, got, want)
	}
}

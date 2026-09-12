package main

import (
	"io"
	"math"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestParseOptionsAcceptsCurrentNavigation(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-latitude", "13",
		"-longitude", "121",
		"-destination-latitude", "14",
		"-destination-longitude", "120",
		"-at", "2026-09-05T09:00:00Z",
		"-minutes", "60",
		"-current-heading", "321",
		"-current-sail", "Jib",
	}, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if options.initialNavigation == nil {
		t.Fatal("état nautique initial absent")
	}
	if options.initialNavigation.Heading != 321 {
		t.Errorf(
			"cap actuel = %.1f°, attendu 321°",
			options.initialNavigation.Heading,
		)
	}
	if options.initialNavigation.SailName != "Jib" {
		t.Errorf(
			"voile actuelle = %q, attendue Jib",
			options.initialNavigation.SailName,
		)
	}
}

func TestParseInitialNavigationOptions(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		options, err := parseInitialNavigationOptions(math.NaN(), "")
		if err != nil {
			t.Fatalf("absence refusée: %v", err)
		}
		if options != nil {
			t.Fatalf("options = %+v, attendues nil", options)
		}
	})

	tests := []struct {
		name        string
		heading     float64
		sail        string
		wantErrText string
	}{
		{
			name:        "cap seul",
			heading:     321,
			wantErrText: "doivent être fournis ensemble",
		},
		{
			name:        "voile seule",
			heading:     math.NaN(),
			sail:        "Jib",
			wantErrText: "doivent être fournis ensemble",
		},
		{
			name:        "cap hors domaine",
			heading:     360,
			sail:        "Jib",
			wantErrText: "-current-heading invalide",
		},
		{
			name:        "cap non fini",
			heading:     math.Inf(1),
			sail:        "Jib",
			wantErrText: "-current-heading invalide",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseInitialNavigationOptions(
				test.heading,
				test.sail,
			)
			if err == nil ||
				!strings.Contains(err.Error(), test.wantErrText) {
				t.Fatalf(
					"erreur = %v, doit contenir %q",
					err,
					test.wantErrText,
				)
			}
		})
	}
}

func TestResolveInitialNavigationState(t *testing.T) {
	boatPolar := &polar.Polar{
		Sails: []polar.Sail{
			{ID: 1, Name: "Jib"},
			{ID: 3, Name: "Code0"},
		},
	}

	state, err := resolveInitialNavigationState(
		boatPolar,
		&initialNavigationOptions{
			Heading:  321,
			SailName: "jib",
		},
	)
	if err != nil {
		t.Fatalf("résolution refusée: %v", err)
	}
	if state == nil {
		t.Fatal("état résolu absent")
	}
	if state.Heading != 321 || state.SailID != 1 {
		t.Errorf("état résolu = %+v", state)
	}
}

func TestResolveInitialNavigationStateRejectsUnknownSail(t *testing.T) {
	_, err := resolveInitialNavigationState(
		&polar.Polar{
			Sails: []polar.Sail{
				{ID: 1, Name: "Jib"},
				{ID: 3, Name: "Code0"},
			},
		},
		&initialNavigationOptions{
			Heading:  321,
			SailName: "Spi",
		},
	)
	if err == nil {
		t.Fatal("voile inconnue acceptée")
	}
	for _, expected := range []string{
		"voile actuelle",
		"Code0, Jib",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf(
				"erreur = %q, doit contenir %q",
				err,
				expected,
			)
		}
	}
}

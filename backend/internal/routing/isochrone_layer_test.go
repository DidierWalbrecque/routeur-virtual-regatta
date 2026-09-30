package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneLayer(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}

	layer, err := BuildIsochroneLayer(
		context.Background(),
		provider,
		testPolar(),
		IsochroneLayerInput{
			Position:       origin,
			Destination:    destination,
			Instant:        instant,
			Duration:       time.Hour,
			HeadingSpacing: 90,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	if layer.Origin != origin {
		t.Errorf("origine = %+v, attendue %+v", layer.Origin, origin)
	}
	if layer.Destination != destination {
		t.Errorf(
			"destination = %+v, attendue %+v",
			layer.Destination,
			destination,
		)
	}
	if !layer.Instant.Equal(instant) {
		t.Errorf("instant = %s, attendu %s", layer.Instant, instant)
	}
	if layer.Duration != time.Hour {
		t.Errorf("durée = %s, attendue 1h", layer.Duration)
	}
	if len(layer.Candidates) != 4 {
		t.Fatalf(
			"nombre de candidats = %d, attendu 4",
			len(layer.Candidates),
		)
	}
	if provider.calls != 1 {
		t.Fatalf("appels météo = %d, attendus 1", provider.calls)
	}

	best, ok := layer.Best()
	if !ok {
		t.Fatal("meilleur candidat absent")
	}
	if best.Candidate.Heading != 90 {
		t.Errorf(
			"meilleur cap = %.1f°, attendu 90°",
			best.Candidate.Heading,
		)
	}
	if best.Progress <= 0 {
		t.Errorf(
			"progression du meilleur cap = %.6f NM, attendue positive",
			best.Progress,
		)
	}
	if best.Candidate.Step.BestSail.SailName != "Jib" {
		t.Errorf(
			"voile du meilleur cap = %q, attendue Jib",
			best.Candidate.Step.BestSail.SailName,
		)
	}
	if !best.Candidate.Step.NextInstant.Equal(instant.Add(time.Hour)) {
		t.Errorf(
			"instant suivant = %s, attendu %s",
			best.Candidate.Step.NextInstant,
			instant.Add(time.Hour),
		)
	}

	last := layer.Candidates[len(layer.Candidates)-1]
	if last.Candidate.Heading != 270 {
		t.Errorf(
			"dernier cap = %.1f°, attendu 270°",
			last.Candidate.Heading,
		)
	}
	if last.Progress >= 0 {
		t.Errorf(
			"progression du dernier cap = %.6f NM, attendue négative",
			last.Progress,
		)
	}
}

func TestIsochroneLayerBestRejectsEmptyLayer(t *testing.T) {
	if _, ok := (IsochroneLayer{}).Best(); ok {
		t.Fatal("une couche vide ne doit pas avoir de meilleur candidat")
	}
}

func TestBuildIsochroneLayerReportsStageErrors(t *testing.T) {
	validInput := IsochroneLayerInput{
		Position:       navigation.Position{},
		Destination:    navigation.Position{Longitude: 10},
		Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Duration:       10 * time.Minute,
		HeadingSpacing: 90,
	}
	validProvider := func() *staticWeatherProvider {
		return &staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		}
	}

	tests := []struct {
		name     string
		input    IsochroneLayerInput
		provider *staticWeatherProvider
		wantErr  string
	}{
		{
			name: "destination invalide",
			input: func() IsochroneLayerInput {
				input := validInput
				input.Destination = navigation.Position{Latitude: 91}
				return input
			}(),
			provider: validProvider(),
			wantErr:  "destination invalide",
		},
		{
			name: "grille invalide",
			input: func() IsochroneLayerInput {
				input := validInput
				input.HeadingSpacing = 7
				return input
			}(),
			provider: validProvider(),
			wantErr:  "grille de caps",
		},
		{
			name:     "évaluation impossible",
			input:    validInput,
			provider: &staticWeatherProvider{err: errors.New("météo indisponible")},
			wantErr:  "évaluation des caps",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildIsochroneLayer(
				context.Background(),
				test.provider,
				testPolar(),
				test.input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

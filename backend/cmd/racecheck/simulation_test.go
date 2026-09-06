package main

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

type weatherCall struct {
	position navigation.Position
	instant  time.Time
}

type trackingWeatherProvider struct {
	calls  []weatherCall
	wind   navigation.Wind
	failAt int
}

func (provider *trackingWeatherProvider) WindAt(
	_ context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	provider.calls = append(provider.calls, weatherCall{
		position: position,
		instant:  instant,
	})
	if provider.failAt > 0 && len(provider.calls) == provider.failAt {
		return navigation.Wind{}, errors.New("météo indisponible")
	}
	return provider.wind, nil
}

func TestSimulateRaceRecalculatesEveryStep(t *testing.T) {
	start := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{
		Latitude:  13.121111,
		Longitude: 121.586667,
	}
	provider := &trackingWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}

	result, err := simulateRace(
		context.Background(),
		provider,
		testPolar(),
		raceSimulationInput{
			Position:    position,
			Instant:     start,
			Heading:     90,
			Minutes:     25,
			StepMinutes: 10,
		},
	)
	if err != nil {
		t.Fatalf("simulation refusée: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("nombre de pas = %d, attendu 3", len(result.Steps))
	}
	if len(provider.calls) != 3 {
		t.Fatalf("appels météo = %d, attendus 3", len(provider.calls))
	}

	wantInstants := []time.Time{
		start,
		start.Add(10 * time.Minute),
		start.Add(20 * time.Minute),
	}
	for index, want := range wantInstants {
		if !provider.calls[index].instant.Equal(want) {
			t.Errorf(
				"instant météo du pas %d = %s, attendu %s",
				index+1,
				provider.calls[index].instant,
				want,
			)
		}
	}
	if provider.calls[1].position == provider.calls[0].position ||
		provider.calls[2].position == provider.calls[1].position {
		t.Fatal("la météo doit être recalculée aux positions successives")
	}

	wantDurations := []time.Duration{
		10 * time.Minute,
		10 * time.Minute,
		5 * time.Minute,
	}
	stepStart := start
	for index, want := range wantDurations {
		got := result.Steps[index].NextInstant.Sub(stepStart)
		if got != want {
			t.Errorf(
				"durée du pas %d = %s, attendue %s",
				index+1,
				got,
				want,
			)
		}
		stepStart = result.Steps[index].NextInstant
	}

	assertClose(
		t,
		"distance totale",
		float64(result.Distance),
		8*25.0/60.0,
	)
	wantEnd := start.Add(25 * time.Minute)
	if !result.NextInstant.Equal(wantEnd) {
		t.Errorf(
			"instant final = %s, attendu %s",
			result.NextInstant,
			wantEnd,
		)
	}
	if result.NextPosition != result.Steps[2].NextPosition {
		t.Fatal("la position finale ne correspond pas au dernier pas")
	}
}

func TestSimulateRaceUsesOneShortStep(t *testing.T) {
	provider := &trackingWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	start := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)

	result, err := simulateRace(
		context.Background(),
		provider,
		testPolar(),
		raceSimulationInput{
			Position:    navigation.Position{},
			Instant:     start,
			Heading:     90,
			Minutes:     5,
			StepMinutes: 10,
		},
	)
	if err != nil {
		t.Fatalf("simulation refusée: %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("nombre de pas = %d, attendu 1", len(result.Steps))
	}
	if got := result.NextInstant.Sub(start); got != 5*time.Minute {
		t.Fatalf("durée simulée = %s, attendue 5m", got)
	}
}

func TestSimulateRaceRejectsInvalidDurations(t *testing.T) {
	provider := &trackingWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	base := raceSimulationInput{
		Position:    navigation.Position{},
		Instant:     time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Heading:     90,
		Minutes:     10,
		StepMinutes: 5,
	}

	for _, test := range []struct {
		name    string
		minutes float64
		step    float64
		wantErr string
	}{
		{name: "durée totale nulle", minutes: 0, step: 5, wantErr: "durée totale"},
		{name: "pas nul", minutes: 10, step: 0, wantErr: "durée d'un pas"},
		{name: "pas non fini", minutes: 10, step: math.Inf(1), wantErr: "durée d'un pas"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.Minutes = test.minutes
			input.StepMinutes = test.step
			_, err := simulateRace(
				context.Background(),
				provider,
				testPolar(),
				input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestSimulateRaceReportsFailingStep(t *testing.T) {
	provider := &trackingWeatherProvider{
		wind:   navigation.Wind{Speed: 5, Direction: 180},
		failAt: 2,
	}

	_, err := simulateRace(
		context.Background(),
		provider,
		testPolar(),
		raceSimulationInput{
			Position:    navigation.Position{},
			Instant:     time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:     90,
			Minutes:     20,
			StepMinutes: 10,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "pas 2") {
		t.Fatalf("erreur = %v, doit identifier le pas 2", err)
	}
}

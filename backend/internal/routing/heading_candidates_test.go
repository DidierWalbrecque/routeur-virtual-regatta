package routing

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

type staticWeatherProvider struct {
	wind  navigation.Wind
	err   error
	calls int
}

func (provider *staticWeatherProvider) WindAt(
	context.Context,
	navigation.Position,
	time.Time,
) (navigation.Wind, error) {
	provider.calls++
	return provider.wind, provider.err
}

func TestEvaluateHeadingCandidates(t *testing.T) {
	position := navigation.Position{
		Latitude:  13.121111,
		Longitude: 121.586667,
	}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	headings := []navigation.Heading{270, 0, 90}
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}

	candidates, err := EvaluateHeadingCandidates(
		context.Background(),
		provider,
		testPolar(),
		HeadingCandidatesInput{
			Position: position,
			Instant:  instant,
			Duration: 10 * time.Minute,
			Headings: headings,
		},
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	if len(candidates) != len(headings) {
		t.Fatalf(
			"nombre de candidats = %d, attendu %d",
			len(candidates),
			len(headings),
		)
	}
	if provider.calls != 1 {
		t.Fatalf(
			"appels météo = %d, attendus %d",
			provider.calls,
			1,
		)
	}

	positions := make(map[navigation.Position]struct{}, len(candidates))
	for index, candidate := range candidates {
		if candidate.Heading != headings[index] {
			t.Errorf(
				"cap du candidat %d = %.1f°, attendu %.1f°",
				index+1,
				candidate.Heading,
				headings[index],
			)
		}
		if candidate.Step.Heading != candidate.Heading {
			t.Errorf(
				"cap simulé du candidat %d = %.1f°",
				index+1,
				candidate.Step.Heading,
			)
		}
		if candidate.Step.Position != position {
			t.Errorf(
				"position initiale du candidat %d modifiée",
				index+1,
			)
		}
		if !candidate.Step.Instant.Equal(instant) {
			t.Errorf(
				"instant initial du candidat %d = %s",
				index+1,
				candidate.Step.Instant,
			)
		}
		if candidate.Step.Duration != 10*time.Minute {
			t.Errorf(
				"durée du candidat %d = %s",
				index+1,
				candidate.Step.Duration,
			)
		}
		if candidate.Step.BestSail.SailName != "Jib" {
			t.Errorf(
				"voile du candidat %d = %q",
				index+1,
				candidate.Step.BestSail.SailName,
			)
		}
		assertClose(
			t,
			"vitesse",
			float64(candidate.Step.BestSail.Speed),
			8,
		)
		positions[candidate.Step.NextPosition] = struct{}{}
	}
	if len(positions) != len(candidates) {
		t.Fatalf(
			"positions finales distinctes = %d, attendues %d",
			len(positions),
			len(candidates),
		)
	}
}

func TestEvaluateHeadingCandidatesRejectsInvalidHeadings(t *testing.T) {
	base := HeadingCandidatesInput{
		Position: navigation.Position{},
		Instant:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Duration: 10 * time.Minute,
	}

	for _, test := range []struct {
		name     string
		headings []navigation.Heading
		wantErr  string
	}{
		{
			name:     "liste vide",
			headings: nil,
			wantErr:  "au moins un cap",
		},
		{
			name:     "cap invalide",
			headings: []navigation.Heading{90, 360},
			wantErr:  "candidat 2",
		},
		{
			name:     "cap dupliqué",
			headings: []navigation.Heading{90, 180, 90},
			wantErr:  "dupliqué",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.Headings = test.headings
			_, err := EvaluateHeadingCandidates(
				context.Background(),
				&staticWeatherProvider{
					wind: navigation.Wind{
						Speed:     5,
						Direction: 180,
					},
				},
				testPolar(),
				input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestEvaluateHeadingCandidatesReportsCandidateError(t *testing.T) {
	_, err := EvaluateHeadingCandidates(
		context.Background(),
		&staticWeatherProvider{
			err: errors.New("météo indisponible"),
		},
		testPolar(),
		HeadingCandidatesInput{
			Position: navigation.Position{},
			Instant:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration: 10 * time.Minute,
			Headings: []navigation.Heading{90},
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "candidat 1") ||
		!strings.Contains(err.Error(), "météo indisponible") {
		t.Fatalf("erreur inattendue: %v", err)
	}
}

func testPolar() *polar.Polar {
	return &polar.Polar{
		Label:            "Test boat",
		GlobalSpeedRatio: 1,
		WindSpeeds:       []polar.WindSpeed{0, 20},
		WindAngles:       []polar.TrueWindAngle{0, 180},
		Sails: []polar.Sail{
			{
				ID:   1,
				Name: "Jib",
				Speeds: polar.SpeedGrid{
					Values: [][]polar.BoatSpeed{
						{8, 8},
						{8, 8},
					},
				},
			},
			{
				ID:   2,
				Name: "Spi",
				Speeds: polar.SpeedGrid{
					Values: [][]polar.BoatSpeed{
						{7, 7},
						{7, 7},
					},
				},
			},
		},
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

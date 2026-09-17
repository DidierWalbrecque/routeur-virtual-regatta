package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestEvaluatePracticalHeadingCandidates(t *testing.T) {
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)

	candidates, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		provider,
		testPolar(),
		PracticalHeadingInput{
			Position:     navigation.Position{},
			Destination:  navigation.Position{Longitude: 10},
			Instant:      instant,
			Duration:     2 * time.Hour,
			StepDuration: time.Hour,
			Headings:     []navigation.Heading{0, 90, 180, 270},
		},
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if len(candidates) != 4 {
		t.Fatalf("candidats = %d, attendus 4", len(candidates))
	}
	if provider.calls != 8 {
		t.Errorf("appels météo = %d, attendus 8", provider.calls)
	}

	best := candidates[0]
	if best.Heading != 90 {
		t.Errorf("meilleur cap = %.1f°, attendu 90°", best.Heading)
	}
	if len(best.Simulation.Steps) != 2 {
		t.Errorf(
			"pas du meilleur cap = %d, attendus 2",
			len(best.Simulation.Steps),
		)
	}
	if best.Simulation.NextInstant != instant.Add(2*time.Hour) {
		t.Errorf(
			"instant final = %s, attendu %s",
			best.Simulation.NextInstant,
			instant.Add(2*time.Hour),
		)
	}
	if best.Progress <= 0 {
		t.Errorf(
			"progression = %.6f NM, attendue positive",
			best.Progress,
		)
	}
	if best.RemainingDistance >= best.InitialDistance {
		t.Errorf(
			"distance restante = %.6f NM, distance initiale %.6f NM",
			best.RemainingDistance,
			best.InitialDistance,
		)
	}
	for index := 1; index < len(candidates); index++ {
		if candidates[index-1].Progress < candidates[index].Progress {
			t.Fatal("les caps pratiques ne sont pas classés par progression")
		}
	}
}

func TestEvaluatePracticalHeadingCandidatesRejectsInvalidInputs(t *testing.T) {
	valid := PracticalHeadingInput{
		Position:     navigation.Position{},
		Destination:  navigation.Position{Longitude: 10},
		Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Duration:     2 * time.Hour,
		StepDuration: time.Hour,
		Headings:     []navigation.Heading{90},
	}
	provider := func() *staticWeatherProvider {
		return &staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		}
	}

	tests := []struct {
		name    string
		input   PracticalHeadingInput
		wantErr string
	}{
		{
			name: "destination invalide",
			input: func() PracticalHeadingInput {
				input := valid
				input.Destination = navigation.Position{Latitude: 91}
				return input
			}(),
			wantErr: "destination pratique",
		},
		{
			name: "instant absent",
			input: func() PracticalHeadingInput {
				input := valid
				input.Instant = time.Time{}
				return input
			}(),
			wantErr: "instant initial",
		},
		{
			name: "durée nulle",
			input: func() PracticalHeadingInput {
				input := valid
				input.Duration = 0
				return input
			}(),
			wantErr: "durée pratique",
		},
		{
			name: "pas nul",
			input: func() PracticalHeadingInput {
				input := valid
				input.StepDuration = 0
				return input
			}(),
			wantErr: "durée de pas",
		},
		{
			name: "caps absents",
			input: func() PracticalHeadingInput {
				input := valid
				input.Headings = nil
				return input
			}(),
			wantErr: "au moins un cap pratique",
		},
		{
			name: "cap invalide",
			input: func() PracticalHeadingInput {
				input := valid
				input.Headings = []navigation.Heading{360}
				return input
			}(),
			wantErr: "cap pratique 1",
		},
		{
			name: "cap dupliqué",
			input: func() PracticalHeadingInput {
				input := valid
				input.Headings = []navigation.Heading{90, 90}
				return input
			}(),
			wantErr: "dupliqué",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := EvaluatePracticalHeadingCandidates(
				context.Background(),
				provider(),
				testPolar(),
				test.input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestEvaluatePracticalHeadingCandidatesReportsSimulationError(
	t *testing.T,
) {
	_, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		&staticWeatherProvider{err: errors.New("prévision absente")},
		testPolar(),
		PracticalHeadingInput{
			Position:     navigation.Position{},
			Destination:  navigation.Position{Longitude: 10},
			Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration:     time.Hour,
			StepDuration: time.Hour,
			Headings:     []navigation.Heading{90},
		},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "cap pratique 1") ||
		!strings.Contains(err.Error(), "prévision absente") {
		t.Fatalf("erreur inattendue: %v", err)
	}
}

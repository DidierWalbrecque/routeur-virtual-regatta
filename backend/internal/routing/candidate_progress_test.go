package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestRankCandidatesByProgress(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{
		Latitude:  0,
		Longitude: 10,
	}
	inputHeadings := []navigation.Heading{270, 0, 90}
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
			Position: origin,
			Instant:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration: time.Hour,
			Headings: inputHeadings,
		},
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	ranked, err := RankCandidatesByProgress(destination, candidates)
	if err != nil {
		t.Fatalf("classement refusé: %v", err)
	}
	if len(ranked) != 3 {
		t.Fatalf("nombre de candidats classés = %d, attendu 3", len(ranked))
	}

	wantHeadings := []navigation.Heading{90, 0, 270}
	for index, want := range wantHeadings {
		if ranked[index].Candidate.Heading != want {
			t.Errorf(
				"cap classé %d = %.1f°, attendu %.1f°",
				index+1,
				ranked[index].Candidate.Heading,
				want,
			)
		}
		if ranked[index].InitialDistance != ranked[0].InitialDistance {
			t.Errorf(
				"distance initiale du candidat %d différente",
				index+1,
			)
		}
	}

	if ranked[0].Progress <= 0 {
		t.Errorf(
			"la route vers l'est doit progresser, progression = %.6f NM",
			ranked[0].Progress,
		)
	}
	if ranked[1].Progress >= 0 || ranked[2].Progress >= 0 {
		t.Errorf(
			"les routes nord et ouest doivent s'éloigner: %.6f, %.6f NM",
			ranked[1].Progress,
			ranked[2].Progress,
		)
	}
	if ranked[0].RemainingDistance >= ranked[1].RemainingDistance ||
		ranked[1].RemainingDistance >= ranked[2].RemainingDistance {
		t.Fatalf(
			"distances restantes non ordonnées: %.6f, %.6f, %.6f NM",
			ranked[0].RemainingDistance,
			ranked[1].RemainingDistance,
			ranked[2].RemainingDistance,
		)
	}

	if candidates[0].Heading != inputHeadings[0] ||
		candidates[1].Heading != inputHeadings[1] ||
		candidates[2].Heading != inputHeadings[2] {
		t.Fatal("le classement a modifié la tranche d'entrée")
	}
}

func TestRankCandidatesByProgressPreservesEqualOrder(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	origin := navigation.Position{}
	nextPosition := navigation.Position{Longitude: 0.1}
	candidates := []HeadingCandidate{
		candidateForRanking(45, origin, nextPosition, instant),
		candidateForRanking(90, origin, nextPosition, instant),
		candidateForRanking(135, origin, nextPosition, instant),
	}

	ranked, err := RankCandidatesByProgress(
		navigation.Position{Longitude: 10},
		candidates,
	)
	if err != nil {
		t.Fatalf("classement refusé: %v", err)
	}

	for index, candidate := range candidates {
		if ranked[index].Candidate.Heading != candidate.Heading {
			t.Errorf(
				"cap classé %d = %.1f°, attendu %.1f°",
				index+1,
				ranked[index].Candidate.Heading,
				candidate.Heading,
			)
		}
	}
}

func TestRankCandidatesByProgressRejectsInvalidInputs(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	origin := navigation.Position{}
	valid := candidateForRanking(
		90,
		origin,
		navigation.Position{Longitude: 0.1},
		instant,
	)

	tests := []struct {
		name        string
		destination navigation.Position
		candidates  []HeadingCandidate
		wantErr     string
	}{
		{
			name:        "destination invalide",
			destination: navigation.Position{Latitude: 91},
			candidates:  []HeadingCandidate{valid},
			wantErr:     "destination invalide",
		},
		{
			name:        "liste vide",
			destination: navigation.Position{Longitude: 10},
			candidates:  nil,
			wantErr:     "au moins un candidat",
		},
		{
			name:        "cap incohérent",
			destination: navigation.Position{Longitude: 10},
			candidates: []HeadingCandidate{
				{
					Heading: 180,
					Step:    valid.Step,
				},
			},
			wantErr: "cap simulé",
		},
		{
			name:        "état initial différent",
			destination: navigation.Position{Longitude: 10},
			candidates: []HeadingCandidate{
				valid,
				candidateForRanking(
					180,
					navigation.Position{Latitude: 1},
					navigation.Position{Latitude: 1.1},
					instant,
				),
			},
			wantErr: "même état initial",
		},
		{
			name:        "position finale invalide",
			destination: navigation.Position{Longitude: 10},
			candidates: []HeadingCandidate{
				candidateForRanking(
					90,
					origin,
					navigation.Position{Latitude: 91},
					instant,
				),
			},
			wantErr: "position finale",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RankCandidatesByProgress(
				test.destination,
				test.candidates,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func candidateForRanking(
	heading navigation.Heading,
	origin navigation.Position,
	nextPosition navigation.Position,
	instant time.Time,
) HeadingCandidate {
	return HeadingCandidate{
		Heading: heading,
		Step: simulation.StepResult{
			Position:     origin,
			Instant:      instant,
			Heading:      heading,
			Duration:     10 * time.Minute,
			NextPosition: nextPosition,
			NextInstant:  instant.Add(10 * time.Minute),
		},
	}
}

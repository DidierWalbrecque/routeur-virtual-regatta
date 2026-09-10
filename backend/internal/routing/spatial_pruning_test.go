package routing

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestPruneCandidatesSpatially(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)

	near := spatialProgressCandidate(
		t,
		10,
		origin,
		navigation.Position{Longitude: 0.02},
		destination,
		instant,
	)
	farCell := spatialProgressCandidate(
		t,
		20,
		origin,
		navigation.Position{Longitude: 0.3},
		destination,
		instant,
	)
	betterSameCell := spatialProgressCandidate(
		t,
		30,
		origin,
		navigation.Position{Longitude: 0.05},
		destination,
		instant,
	)
	input := []CandidateProgress{near, farCell, betterSameCell}

	pruned, err := PruneCandidatesSpatially(input, 10)
	if err != nil {
		t.Fatalf("élagage refusé: %v", err)
	}

	if len(pruned) != 2 {
		t.Fatalf("nombre de candidats = %d, attendu 2", len(pruned))
	}
	if pruned[0].Candidate.Heading != farCell.Candidate.Heading {
		t.Errorf(
			"premier cap = %.1f°, attendu %.1f°",
			pruned[0].Candidate.Heading,
			farCell.Candidate.Heading,
		)
	}
	if pruned[1].Candidate.Heading != betterSameCell.Candidate.Heading {
		t.Errorf(
			"second cap = %.1f°, attendu %.1f°",
			pruned[1].Candidate.Heading,
			betterSameCell.Candidate.Heading,
		)
	}
	if input[0].Candidate.Heading != near.Candidate.Heading {
		t.Fatal("la tranche d'entrée a été modifiée")
	}
}

func TestPruneCandidatesSpatiallyKeepsFirstOnTie(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	nextPosition := navigation.Position{Longitude: 0.02}

	first := spatialProgressCandidate(
		t,
		45,
		origin,
		nextPosition,
		destination,
		instant,
	)
	second := spatialProgressCandidate(
		t,
		90,
		origin,
		nextPosition,
		destination,
		instant,
	)

	pruned, err := PruneCandidatesSpatially(
		[]CandidateProgress{first, second},
		10,
	)
	if err != nil {
		t.Fatalf("élagage refusé: %v", err)
	}
	if len(pruned) != 1 {
		t.Fatalf("nombre de candidats = %d, attendu 1", len(pruned))
	}
	if pruned[0].Candidate.Heading != first.Candidate.Heading {
		t.Errorf(
			"cap conservé = %.1f°, attendu %.1f°",
			pruned[0].Candidate.Heading,
			first.Candidate.Heading,
		)
	}
}

func TestPruneCandidatesSpatiallyHandlesAntimeridian(t *testing.T) {
	origin := navigation.Position{Longitude: 179.9}
	destination := navigation.Position{Longitude: -170}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)

	nearOrigin := spatialProgressCandidate(
		t,
		90,
		origin,
		navigation.Position{Longitude: 179.95},
		destination,
		instant,
	)
	fartherEast := spatialProgressCandidate(
		t,
		100,
		origin,
		navigation.Position{Longitude: -179.98},
		destination,
		instant,
	)

	pruned, err := PruneCandidatesSpatially(
		[]CandidateProgress{nearOrigin, fartherEast},
		10,
	)
	if err != nil {
		t.Fatalf("élagage refusé près de l'antiméridien: %v", err)
	}
	if len(pruned) != 1 {
		t.Fatalf("nombre de cellules = %d, attendu 1", len(pruned))
	}
	if pruned[0].Candidate.Heading != fartherEast.Candidate.Heading {
		t.Errorf(
			"cap conservé = %.1f°, attendu %.1f°",
			pruned[0].Candidate.Heading,
			fartherEast.Candidate.Heading,
		)
	}
}

func TestIsochroneLayerPruneSpatially(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	candidates := []CandidateProgress{
		spatialProgressCandidate(
			t,
			10,
			origin,
			navigation.Position{Longitude: 0.02},
			destination,
			instant,
		),
		spatialProgressCandidate(
			t,
			20,
			origin,
			navigation.Position{Longitude: 0.3},
			destination,
			instant,
		),
		spatialProgressCandidate(
			t,
			30,
			origin,
			navigation.Position{Longitude: 0.05},
			destination,
			instant,
		),
	}
	layer := IsochroneLayer{
		Origin:      origin,
		Destination: destination,
		Instant:     instant,
		Duration:    10 * time.Minute,
		Candidates:  candidates,
	}

	pruned, err := layer.PruneSpatially(10)
	if err != nil {
		t.Fatalf("élagage de couche refusé: %v", err)
	}
	if len(pruned.Candidates) != 2 {
		t.Fatalf(
			"candidats élagués = %d, attendus 2",
			len(pruned.Candidates),
		)
	}
	if len(layer.Candidates) != 3 {
		t.Fatal("la couche d'origine a été modifiée")
	}
}

func TestPruneCandidatesSpatiallyRejectsInvalidInputs(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	valid := spatialProgressCandidate(
		t,
		90,
		origin,
		navigation.Position{Longitude: 0.02},
		destination,
		instant,
	)
	inconsistent := valid
	inconsistent.Progress++

	tests := []struct {
		name       string
		candidates []CandidateProgress
		cellSize   navigation.NauticalMiles
		wantErr    string
	}{
		{
			name:       "liste vide",
			candidates: nil,
			cellSize:   10,
			wantErr:    "au moins un candidat",
		},
		{
			name:       "taille nulle",
			candidates: []CandidateProgress{valid},
			cellSize:   0,
			wantErr:    "strictement positif",
		},
		{
			name:       "taille non finie",
			candidates: []CandidateProgress{valid},
			cellSize:   navigation.NauticalMiles(math.NaN()),
			wantErr:    "nombre fini",
		},
		{
			name:       "progression incohérente",
			candidates: []CandidateProgress{inconsistent},
			cellSize:   10,
			wantErr:    "progression incohérente",
		},
		{
			name:       "cellule trop petite",
			candidates: []CandidateProgress{valid},
			cellSize: navigation.NauticalMiles(
				math.SmallestNonzeroFloat64,
			),
			wantErr: "trop petite",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := PruneCandidatesSpatially(
				test.candidates,
				test.cellSize,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func spatialProgressCandidate(
	t *testing.T,
	heading navigation.Heading,
	origin navigation.Position,
	nextPosition navigation.Position,
	destination navigation.Position,
	instant time.Time,
) CandidateProgress {
	t.Helper()

	initialDistance, err := navigation.DistanceBetween(origin, destination)
	if err != nil {
		t.Fatalf("distance initiale impossible: %v", err)
	}
	remainingDistance, err := navigation.DistanceBetween(
		nextPosition,
		destination,
	)
	if err != nil {
		t.Fatalf("distance restante impossible: %v", err)
	}

	return CandidateProgress{
		Candidate: candidateForRanking(
			heading,
			origin,
			nextPosition,
			instant,
		),
		InitialDistance:   initialDistance,
		RemainingDistance: remainingDistance,
		Progress: DistanceProgress(
			float64(initialDistance - remainingDistance),
		),
	}
}

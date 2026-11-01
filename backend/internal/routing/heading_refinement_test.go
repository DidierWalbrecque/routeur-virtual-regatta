package routing

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestRefinementStepsReachHalfTheSpacing(t *testing.T) {
	for _, spacing := range []float64{3, 5, 10, 12, 15, 20, 30, 45} {
		steps := refinementSteps(spacing)
		if len(steps) == 0 {
			t.Fatalf("pas %v° : aucun palier d'affinage", spacing)
		}
		reach := 0
		for index, step := range steps {
			if step < 1 {
				t.Fatalf("pas %v° : palier %d invalide", spacing, step)
			}
			if index > 0 && step > steps[index-1] {
				t.Fatalf("pas %v° : les paliers ne doivent pas croître, obtenu %v", spacing, steps)
			}
			reach += step
		}
		if steps[len(steps)-1] != 1 {
			t.Fatalf("pas %v° : le dernier palier doit valoir 1°, obtenu %v", spacing, steps)
		}
		if float64(reach) < spacing/2 {
			t.Fatalf("pas %v° : portée %d° inférieure à la moitié du pas (%v)", spacing, reach, steps)
		}
	}
	for _, spacing := range []float64{1, 2, 2.9} {
		if steps := refinementSteps(spacing); steps != nil {
			t.Fatalf("pas %v° : aucun affinage attendu, obtenu %v", spacing, steps)
		}
	}
}

func TestRefineHeadingCandidatesFindsTheIntegerOptimum(t *testing.T) {
	origin := navigation.Position{Latitude: 0, Longitude: 0}
	// La polaire de test ne dépend pas de l'angle : le meilleur cap est celui
	// qui pointe vers la cible, ici environ 83,4° (hors de la grille de 15°).
	target, err := navigation.DestinationPosition(origin, 83.4, 100)
	if err != nil {
		t.Fatal(err)
	}
	provider := &staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}}
	input := HeadingCandidatesInput{
		Position: origin,
		Instant:  time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Duration: 10 * time.Minute,
	}

	best := func(headings []navigation.Heading) navigation.Heading {
		candidateInput := input
		candidateInput.Headings = headings
		candidates, evalErr := EvaluateHeadingCandidates(context.Background(), provider, testPolar(), candidateInput)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		return bestProgressHeading(t, candidates, origin, target)
	}

	grid, err := GenerateHeadingGrid(15)
	if err != nil {
		t.Fatal(err)
	}
	input.Headings = grid
	coarse, err := EvaluateHeadingCandidates(context.Background(), provider, testPolar(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got := bestProgressHeading(t, coarse, origin, target); got != 75 && got != 90 {
		t.Fatalf("la grille de 15° doit donner 75° ou 90°, obtenu %v", got)
	}

	refined, err := refineHeadingCandidates(context.Background(), provider, testPolar(), input, coarse, target, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(refined) <= len(coarse) {
		t.Fatalf("l'affinage doit ajouter des caps : %d → %d", len(coarse), len(refined))
	}
	seen := map[navigation.Heading]bool{}
	for _, candidate := range refined {
		if seen[candidate.Heading] {
			t.Fatalf("cap évalué deux fois : %v", candidate.Heading)
		}
		seen[candidate.Heading] = true
	}

	every := make([]navigation.Heading, 360)
	for degree := range every {
		every[degree] = navigation.Heading(degree)
	}
	wanted := best(every)
	got := bestProgressHeading(t, refined, origin, target)
	if got != navigation.Heading(math.Round(float64(got))) {
		t.Fatalf("le cap affiné doit être entier, obtenu %v", got)
	}
	if math.Abs(float64(got-wanted)) > 1 {
		t.Fatalf("cap affiné %v, optimum au degré près %v", got, wanted)
	}
}

func TestRefineHeadingCandidatesLeavesFineGridsUntouched(t *testing.T) {
	origin := navigation.Position{Latitude: 0, Longitude: 0}
	target := navigation.Position{Latitude: 0, Longitude: 2}
	provider := &staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}}
	grid, err := GenerateHeadingGrid(2)
	if err != nil {
		t.Fatal(err)
	}
	input := HeadingCandidatesInput{
		Position: origin,
		Instant:  time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Duration: 10 * time.Minute,
		Headings: grid,
	}
	candidates, err := EvaluateHeadingCandidates(context.Background(), provider, testPolar(), input)
	if err != nil {
		t.Fatal(err)
	}
	refined, err := refineHeadingCandidates(context.Background(), provider, testPolar(), input, candidates, target, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(refined) != len(candidates) {
		t.Fatalf("aucun cap ajouté attendu pour un pas de 2°, %d → %d", len(candidates), len(refined))
	}
}

func bestProgressHeading(
	t *testing.T,
	candidates []HeadingCandidate,
	origin,
	target navigation.Position,
) navigation.Heading {
	t.Helper()
	initial, err := navigation.DistanceBetween(origin, target)
	if err != nil {
		t.Fatal(err)
	}
	best := candidates[0].Heading
	bestProgress := math.Inf(-1)
	for _, candidate := range candidates {
		remaining, distanceErr := navigation.DistanceBetween(candidate.Step.NextPosition, target)
		if distanceErr != nil {
			t.Fatal(distanceErr)
		}
		if progress := float64(initial - remaining); progress > bestProgress {
			best, bestProgress = candidate.Heading, progress
		}
	}
	return best
}

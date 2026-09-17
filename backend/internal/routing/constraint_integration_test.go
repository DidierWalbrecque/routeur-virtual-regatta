package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneSearchFiltersForbiddenSegments(t *testing.T) {
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	calls := 0
	eastOnly := ConstraintFunc(func(
		_ context.Context,
		segment RouteSegment,
	) (bool, error) {
		calls++
		return segment.To.Longitude > segment.From.Longitude+0.01, nil
	})

	search, err := BuildIsochroneSearch(
		context.Background(),
		provider,
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     1,
			HeadingSpacing: 90,
			CellSize:       1,
			Constraints:    []Constraint{eastOnly},
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if calls != 4 {
		t.Errorf("segments vérifiés = %d, attendus 4", calls)
	}
	if len(search.Frontiers) != 1 {
		t.Fatalf("fronts = %d, attendu 1", len(search.Frontiers))
	}
	nodes := search.Frontiers[0].Nodes
	if len(nodes) != 1 {
		t.Fatalf("positions conservées = %d, attendue 1", len(nodes))
	}
	if nodes[0].Candidate.Heading != 90 {
		t.Errorf(
			"cap conservé = %.3f°, attendu 90°",
			nodes[0].Candidate.Heading,
		)
	}
}

func TestBuildIsochroneSearchReportsBlockedFrontier(t *testing.T) {
	denyAll := ConstraintFunc(func(
		context.Context,
		RouteSegment,
	) (bool, error) {
		return false, nil
	})

	_, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{Speed: 5, Direction: 180},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     1,
			HeadingSpacing: 90,
			CellSize:       1,
			Constraints:    []Constraint{denyAll},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "aucune trajectoire") {
		t.Fatalf("erreur = %v, front bloqué attendu", err)
	}
}

func TestBuildIsochroneSearchPropagatesConstraintError(t *testing.T) {
	failing := ConstraintFunc(func(
		context.Context,
		RouteSegment,
	) (bool, error) {
		return false, errors.New("source indisponible")
	})

	_, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{Speed: 5, Direction: 180},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     1,
			HeadingSpacing: 90,
			CellSize:       1,
			Constraints:    []Constraint{failing},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "source indisponible") {
		t.Fatalf("erreur = %v, erreur de contrainte attendue", err)
	}
}

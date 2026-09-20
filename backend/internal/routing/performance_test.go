package routing

import (
	"context"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func BenchmarkBuildIsochroneSearch(b *testing.B) {
	input := benchmarkIsochroneInput(nil)
	provider := &staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}}
	for index := 0; index < b.N; index++ {
		if _, err := BuildIsochroneSearch(context.Background(), provider, testPolar(), input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildIsochroneSearchWithConstraint(b *testing.B) {
	constraint := ConstraintFunc(func(_ context.Context, segment RouteSegment) (bool, error) {
		// Synthetic island used only to keep the benchmark deterministic.
		crossesIsland := segment.From.Longitude < 2 && segment.To.Longitude > 2 &&
			segment.From.Latitude > -1 && segment.From.Latitude < 1
		return !crossesIsland, nil
	})
	input := benchmarkIsochroneInput([]Constraint{constraint})
	provider := &staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}}
	for index := 0; index < b.N; index++ {
		if _, err := BuildIsochroneSearch(context.Background(), provider, testPolar(), input); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkIsochroneInput(constraints []Constraint) IsochroneSearchInput {
	return IsochroneSearchInput{
		Position:       navigation.Position{Latitude: 0, Longitude: 0},
		Destination:    navigation.Position{Latitude: 0, Longitude: 20},
		Instant:        time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		StepDuration:   10 * time.Minute,
		LayerCount:     24,
		HeadingSpacing: 15,
		CellSize:       0.5,
		MaxCandidates:  100,
		Constraints:    constraints,
	}
}

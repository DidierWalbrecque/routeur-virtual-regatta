package routing

import (
	"context"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneSearchTargetsNextMandatoryGate(t *testing.T) {
	gateTarget := navigation.Position{Latitude: 1}
	search, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
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
			Gates: []CourseGate{{
				Name:       "porte nord",
				FirstMark:  navigation.Position{Latitude: 1, Longitude: -0.5},
				SecondMark: navigation.Position{Latitude: 1, Longitude: 0.5},
			}},
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	best, ok := search.Best()
	if !ok {
		t.Fatal("meilleur nœud absent")
	}
	if best.Candidate.Heading != 0 {
		t.Errorf(
			"meilleur cap = %.1f°, attendu 0° vers la porte",
			best.Candidate.Heading,
		)
	}
	if best.CourseTarget != gateTarget {
		t.Errorf(
			"cible de parcours = %+v, attendue %+v",
			best.CourseTarget,
			gateTarget,
		)
	}
	if best.CourseProgress.NextGateIndex != 0 {
		t.Errorf(
			"prochaine porte = %d, attendue 0",
			best.CourseProgress.NextGateIndex,
		)
	}
	if best.TargetProgress <= 0 {
		t.Errorf(
			"progression vers la porte = %.6f NM, attendue positive",
			best.TargetProgress,
		)
	}
	if best.TargetRemainingDistance >= best.TargetInitialDistance {
		t.Errorf(
			"distance restante vers la porte = %.6f NM, distance initiale %.6f NM",
			best.TargetRemainingDistance,
			best.TargetInitialDistance,
		)
	}
}

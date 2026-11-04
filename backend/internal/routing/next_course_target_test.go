package routing

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneSearchTargetsNextMandatoryGate(t *testing.T) {
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
	// La cible est désormais le point le plus proche sur la ligne de porte
	// (et non son milieu fixe) : il reste sur la latitude de la porte et dans
	// l'étendue de ses marques.
	if math.Abs(float64(best.CourseTarget.Latitude)-1) > 1e-6 ||
		float64(best.CourseTarget.Longitude) < -0.5-1e-6 ||
		float64(best.CourseTarget.Longitude) > 0.5+1e-6 {
		t.Errorf(
			"cible de parcours = %+v, attendue sur la ligne de porte (lat 1, lon dans [-0.5, 0.5])",
			best.CourseTarget,
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

func TestCourseGateClosestPointStaysOnSegment(t *testing.T) {
	gate := CourseGate{
		Name:       "porte",
		FirstMark:  navigation.Position{Latitude: 1, Longitude: -1},
		SecondMark: navigation.Position{Latitude: 1, Longitude: 1},
	}
	// Un point sous la porte se projette au même endroit le long de la ligne.
	inside, err := gate.ClosestPoint(navigation.Position{Latitude: 0, Longitude: 0.8})
	if err != nil {
		t.Fatalf("projection impossible: %v", err)
	}
	if math.Abs(float64(inside.Latitude)-1) > 1e-6 ||
		math.Abs(float64(inside.Longitude)-0.8) > 1e-3 {
		t.Errorf("point projeté = %+v, attendu proche de {1, 0.8}", inside)
	}
	// Au-delà de la seconde marque, la projection est bornée à cette marque.
	clamped, err := gate.ClosestPoint(navigation.Position{Latitude: 0, Longitude: 5})
	if err != nil {
		t.Fatalf("projection impossible: %v", err)
	}
	if math.Abs(float64(clamped.Longitude)-1) > 1e-6 {
		t.Errorf("point projeté = %+v, attendu la seconde marque (lon 1)", clamped)
	}
}

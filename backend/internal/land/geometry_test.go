package land

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func TestGeoJSONConstraintAllowsWaterSegment(t *testing.T) {
	constraint := squareLandConstraint()
	allowed, err := constraint.Allows(
		context.Background(),
		testSegment(-5, -5, -2, -2),
	)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if !allowed {
		t.Fatal("segment maritime refusé")
	}
}

func BenchmarkGeoJSONConstraintAllows(b *testing.B) {
	constraint, err := squareLandConstraint().WithCoastalMargin(2)
	if err != nil {
		b.Fatal(err)
	}
	segment := testSegment(-0.05, 5, -0.05, 6)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		allowed, err := constraint.Allows(context.Background(), segment)
		if err != nil {
			b.Fatal(err)
		}
		if !allowed {
			b.Fatal("segment maritime refusé")
		}
	}
}

func TestGeoJSONConstraintRejectsLandSegments(t *testing.T) {
	constraint := squareLandConstraint()
	tests := []struct {
		name    string
		segment routing.RouteSegment
	}{
		{
			name:    "traverse le polygone",
			segment: testSegment(-1, 5, 11, 5),
		},
		{
			name:    "démarre dans le polygone",
			segment: testSegment(5, 5, 12, 5),
		},
		{
			name:    "touche la côte",
			segment: testSegment(-1, 0, 0, 0),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, err := constraint.Allows(
				context.Background(),
				test.segment,
			)
			if err != nil {
				t.Fatalf("vérification refusée: %v", err)
			}
			if allowed {
				t.Fatal("segment terrestre autorisé")
			}
		})
	}
}

func TestGeoJSONConstraintAllowsSegmentInsideWaterHole(t *testing.T) {
	constraint := &GeoJSONConstraint{
		polygons: []polygon{
			{
				rings: [][]point{
					squareRing(0, 0, 10, 10),
					squareRing(3, 3, 7, 7),
				},
			},
		},
	}
	allowed, err := constraint.Allows(
		context.Background(),
		testSegment(4, 4, 6, 4),
	)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if !allowed {
		t.Fatal("segment situé dans un trou maritime refusé")
	}
}

func TestGeoJSONConstraintHandlesAntimeridian(t *testing.T) {
	constraint := &GeoJSONConstraint{
		polygons: []polygon{
			{
				rings: [][]point{
					{
						{longitude: 179, latitude: -1},
						{longitude: -179, latitude: -1},
						{longitude: -179, latitude: 1},
						{longitude: 179, latitude: 1},
						{longitude: 179, latitude: -1},
					},
				},
			},
		},
	}

	allowed, err := constraint.Allows(
		context.Background(),
		testSegment(178, 0, -178, 0),
	)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if allowed {
		t.Fatal("traversée terrestre de l'antiméridien autorisée")
	}

	allowed, err = constraint.Allows(
		context.Background(),
		testSegment(170, 0, 171, 0),
	)
	if err != nil {
		t.Fatalf("vérification maritime refusée: %v", err)
	}
	if !allowed {
		t.Fatal("segment éloigné de l'antiméridien refusé")
	}
}

func TestGeoJSONConstraintAppliesCoastalMargin(t *testing.T) {
	base := squareLandConstraint()
	constraint, err := base.WithCoastalMargin(1)
	if err != nil {
		t.Fatalf("marge refusée: %v", err)
	}
	if base.CoastalMargin() != 0 {
		t.Fatalf("marge de la contrainte source = %.3f NM", base.CoastalMargin())
	}
	if constraint.CoastalMargin() != 1 {
		t.Fatalf("marge = %.3f NM, attendue 1", constraint.CoastalMargin())
	}

	tests := []struct {
		name    string
		segment routing.RouteSegment
		allowed bool
	}{
		{
			name:    "moins d'un mille de la côte",
			segment: testSegment(-0.01, 4, -0.01, 6),
		},
		{
			name:    "plus d'un mille de la côte",
			segment: testSegment(-0.05, 4, -0.05, 6),
			allowed: true,
		},
		{
			name:    "départ dans la marge qui s'éloigne de la côte",
			segment: testSegment(-0.01, 5, -0.1, 5),
			allowed: true,
		},
		{
			name:    "départ dans la marge qui se rapproche de la côte",
			segment: testSegment(-0.01, 5, -0.005, 5),
		},
		{
			name:    "départ dans la marge qui longe la côte",
			segment: testSegment(-0.01, 5, -0.01, 5.1),
		},
		{
			name:    "départ dans la marge qui traverse la terre",
			segment: testSegment(-0.01, 5, 10.5, 5),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, err := constraint.Allows(
				context.Background(),
				test.segment,
			)
			if err != nil {
				t.Fatalf("vérification refusée: %v", err)
			}
			if allowed != test.allowed {
				t.Fatalf(
					"autorisé = %t, attendu %t",
					allowed,
					test.allowed,
				)
			}
		})
	}
}

func TestGeoJSONConstraintRejectsInvalidCoastalMargins(t *testing.T) {
	constraint := squareLandConstraint()
	for _, margin := range []navigation.NauticalMiles{
		-1,
		navigation.NauticalMiles(math.NaN()),
		navigation.NauticalMiles(math.Inf(1)),
	} {
		if _, err := constraint.WithCoastalMargin(margin); err == nil {
			t.Fatalf("marge %.12f acceptée", margin)
		}
	}

	var missing *GeoJSONConstraint
	if _, err := missing.WithCoastalMargin(1); err == nil {
		t.Fatal("contrainte absente acceptée")
	}
}

func TestGeoJSONConstraintHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := squareLandConstraint().Allows(
		ctx,
		testSegment(-5, -5, -2, -2),
	)
	if err == nil {
		t.Fatal("annulation attendue")
	}
}

func squareLandConstraint() *GeoJSONConstraint {
	return &GeoJSONConstraint{
		polygons: []polygon{
			{
				rings: [][]point{
					squareRing(0, 0, 10, 10),
				},
			},
		},
	}
}

func squareRing(minLongitude, minLatitude, maxLongitude, maxLatitude float64) []point {
	return []point{
		{longitude: minLongitude, latitude: minLatitude},
		{longitude: maxLongitude, latitude: minLatitude},
		{longitude: maxLongitude, latitude: maxLatitude},
		{longitude: minLongitude, latitude: maxLatitude},
		{longitude: minLongitude, latitude: minLatitude},
	}
}

func testSegment(
	fromLongitude,
	fromLatitude,
	toLongitude,
	toLatitude float64,
) routing.RouteSegment {
	startedAt := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	return routing.RouteSegment{
		From: navigation.Position{
			Latitude:  navigation.Latitude(fromLatitude),
			Longitude: navigation.Longitude(fromLongitude),
		},
		To: navigation.Position{
			Latitude:  navigation.Latitude(toLatitude),
			Longitude: navigation.Longitude(toLongitude),
		},
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(10 * time.Minute),
	}
}

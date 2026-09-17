package land

import (
	"context"
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

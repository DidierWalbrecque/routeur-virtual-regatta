package land

import (
	"reflect"
	"testing"
)

func TestPolygonSpatialIndexNarrowsCandidates(t *testing.T) {
	polygons := []polygon{
		{rings: [][]point{squareRing(0, 0, 1, 1)}},
		{rings: [][]point{squareRing(50, 50, 51, 51)}},
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
	}
	constraint := &GeoJSONConstraint{
		polygons: polygons,
		index:    newPolygonSpatialIndex(polygons),
	}

	tests := []struct {
		name  string
		start point
		end   point
		want  []int
	}{
		{
			name:  "premier polygone",
			start: point{longitude: -1, latitude: 0.5},
			end:   point{longitude: 2, latitude: 0.5},
			want:  []int{0},
		},
		{
			name:  "aucun polygone proche",
			start: point{longitude: 20, latitude: 20},
			end:   point{longitude: 21, latitude: 21},
			want:  []int{},
		},
		{
			name:  "antiméridien",
			start: point{longitude: 178, latitude: 0},
			end:   point{longitude: 182, latitude: 0},
			want:  []int{2},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := constraint.candidatePolygonIndices(test.start, test.end, 0)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("indices = %v, attendus %v", got, test.want)
			}
		})
	}
}

func TestPolygonSpatialIndexIncludesNeighborWithinMargin(t *testing.T) {
	polygons := []polygon{
		{rings: [][]point{squareRing(5.01, 0, 6, 1)}},
	}
	constraint := &GeoJSONConstraint{
		polygons: polygons,
		index:    newPolygonSpatialIndex(polygons),
	}
	start := point{longitude: 4.99, latitude: 0.25}
	end := point{longitude: 4.99, latitude: 0.75}

	withoutMargin := constraint.candidatePolygonIndices(start, end, 0)
	if len(withoutMargin) != 0 {
		t.Fatalf("indices sans marge = %v, attendus vides", withoutMargin)
	}
	withMargin := constraint.candidatePolygonIndices(start, end, 2)
	want := []int{0}
	if !reflect.DeepEqual(withMargin, want) {
		t.Fatalf("indices avec marge = %v, attendus %v", withMargin, want)
	}
}

func TestPolygonSpatialIndexFallbackChecksEveryPolygon(t *testing.T) {
	constraint := &GeoJSONConstraint{
		polygons: []polygon{{}, {}, {}},
	}
	got := constraint.candidatePolygonIndices(point{}, point{}, 0)
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indices = %v, attendus %v", got, want)
	}
}

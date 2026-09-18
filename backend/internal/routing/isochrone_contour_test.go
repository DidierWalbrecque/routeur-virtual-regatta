package routing

import (
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"math"
	"reflect"
	"testing"
)

func contourFixture(points ...navigation.Position) IsochroneFrontier {
	f := IsochroneFrontier{}
	for _, p := range points {
		n := IsochroneNode{}
		n.Candidate.Step.NextPosition = p
		f.Nodes = append(f.Nodes, n)
	}
	return f
}

func TestContourRemovesInteriorSpikesAndClosesBoundary(t *testing.T) {
	points := []navigation.Position{{Latitude: 1, Longitude: 1}, {Latitude: -1, Longitude: -1}, {Latitude: 0.01, Longitude: 0.02}, {Latitude: 1, Longitude: -1}, {Latitude: -1, Longitude: 1}, {Latitude: 1, Longitude: 1}}
	expected := []navigation.Position{{Latitude: -1, Longitude: -1}, {Latitude: -1, Longitude: 1}, {Latitude: 1, Longitude: 1}, {Latitude: 1, Longitude: -1}, {Latitude: -1, Longitude: -1}}
	for iteration := 0; iteration < len(points); iteration++ {
		got, err := contourFixture(points...).Contour(navigation.Position{}, 36)
		if err != nil || !reflect.DeepEqual(got, expected) {
			t.Fatalf("contour = %v, %v; want %v", got, err, expected)
		}
		points = append(points[1:], points[0])
	}
}

func TestContourRejectsDegenerateAndInvalidGeometry(t *testing.T) {
	cases := [][]navigation.Position{nil, {{}}, {{}, {Latitude: 1, Longitude: 1}, {Latitude: 2, Longitude: 2}}, {{Latitude: navigation.Latitude(math.NaN())}}, {{Longitude: 181}}, {{}, {}, {}}}
	for _, points := range cases {
		if _, err := contourFixture(points...).Contour(navigation.Position{}, 36); err == nil {
			t.Fatalf("expected error for %v", points)
		}
	}
}

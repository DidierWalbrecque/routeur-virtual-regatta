package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestInitialHeading(t *testing.T) {
	tests := []struct {
		name string
		from Position
		to   Position
		want Heading
	}{
		{
			name: "plein nord",
			from: Position{Latitude: 0, Longitude: 0},
			to:   Position{Latitude: 1, Longitude: 0},
			want: 0,
		},
		{
			name: "plein est",
			from: Position{Latitude: 0, Longitude: 0},
			to:   Position{Latitude: 0, Longitude: 1},
			want: 90,
		},
		{
			name: "plein sud",
			from: Position{Latitude: 0, Longitude: 0},
			to:   Position{Latitude: -1, Longitude: 0},
			want: 180,
		},
		{
			name: "plein ouest",
			from: Position{Latitude: 0, Longitude: 0},
			to:   Position{Latitude: 0, Longitude: -1},
			want: 270,
		},
		{
			name: "Paris vers New York",
			from: Position{Latitude: 48.8566, Longitude: 2.3522},
			to:   Position{Latitude: 40.7128, Longitude: -74.0060},
			want: 291.7938627483058,
		},
		{
			name: "antiméridien vers l'est",
			from: Position{Latitude: 0, Longitude: 179},
			to:   Position{Latitude: 0, Longitude: -179},
			want: 90,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := InitialHeading(test.from, test.to)
			if err != nil {
				t.Fatalf(
					"InitialHeading() a retourné une erreur inattendue: %v",
					err,
				)
			}

			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf(
					"InitialHeading() = %.12f, attendu %.12f",
					got,
					test.want,
				)
			}
			if err := got.Validate(); err != nil {
				t.Errorf("le cap retourné doit être valide: %v", err)
			}
		})
	}
}

func TestInitialHeadingRejectsUndefinedRoutes(t *testing.T) {
	tests := []struct {
		name      string
		from      Position
		to        Position
		errorPart string
	}{
		{
			name:      "positions identiques",
			from:      Position{Latitude: 48.8566, Longitude: 2.3522},
			to:        Position{Latitude: 48.8566, Longitude: 2.3522},
			errorPart: "indéfini",
		},
		{
			name:      "positions antipodales",
			from:      Position{Latitude: 0, Longitude: 0},
			to:        Position{Latitude: 0, Longitude: 180},
			errorPart: "antipodales",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := InitialHeading(test.from, test.to)
			if err == nil {
				t.Fatal("InitialHeading() doit refuser cette route")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestInitialHeadingRejectsInvalidPositions(t *testing.T) {
	tests := []struct {
		name      string
		from      Position
		to        Position
		errorPart string
	}{
		{
			name:      "départ invalide",
			from:      Position{Latitude: 91},
			to:        Position{},
			errorPart: "position de départ",
		},
		{
			name:      "arrivée invalide",
			from:      Position{},
			to:        Position{Longitude: 181},
			errorPart: "position d'arrivée",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := InitialHeading(test.from, test.to)
			if err == nil {
				t.Fatal("InitialHeading() doit refuser cette position")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestHeadingValidate(t *testing.T) {
	for _, valid := range []Heading{0, 90, 359.999999} {
		if err := valid.Validate(); err != nil {
			t.Errorf("Heading(%v) doit être valide: %v", valid, err)
		}
	}

	for _, invalid := range []Heading{
		-0.1,
		360,
		Heading(math.NaN()),
		Heading(math.Inf(1)),
	} {
		if err := invalid.Validate(); err == nil {
			t.Errorf("Heading(%v) doit être invalide", invalid)
		}
	}
}

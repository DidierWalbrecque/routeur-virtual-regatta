package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestDistanceBetween(t *testing.T) {
	tests := []struct {
		name string
		from Position
		to   Position
		want NauticalMiles
	}{
		{
			name: "positions identiques",
			from: Position{Latitude: 48.8566, Longitude: 2.3522},
			to:   Position{Latitude: 48.8566, Longitude: 2.3522},
			want: 0,
		},
		{
			name: "un degré de latitude à l'équateur",
			from: Position{Latitude: 0, Longitude: 0},
			to:   Position{Latitude: 1, Longitude: 0},
			want: 60.04046073261873,
		},
		{
			name: "Paris vers New York",
			from: Position{Latitude: 48.8566, Longitude: 2.3522},
			to:   Position{Latitude: 40.7128, Longitude: -74.0060},
			want: 3151.858127424209,
		},
		{
			name: "passage de l'antiméridien",
			from: Position{Latitude: 0, Longitude: 179},
			to:   Position{Latitude: 0, Longitude: -179},
			want: 120.08092146523697,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DistanceBetween(test.from, test.to)
			if err != nil {
				t.Fatalf(
					"DistanceBetween() a retourné une erreur inattendue: %v",
					err,
				)
			}

			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf(
					"DistanceBetween() = %.12f, attendu %.12f",
					got,
					test.want,
				)
			}
		})
	}
}

func TestDistanceBetweenIsSymmetrical(t *testing.T) {
	from := Position{Latitude: -33.8688, Longitude: 151.2093}
	to := Position{Latitude: 35.6762, Longitude: 139.6503}

	forward, err := DistanceBetween(from, to)
	if err != nil {
		t.Fatalf("DistanceBetween(from, to) a échoué: %v", err)
	}
	backward, err := DistanceBetween(to, from)
	if err != nil {
		t.Fatalf("DistanceBetween(to, from) a échoué: %v", err)
	}

	if difference := math.Abs(float64(forward - backward)); difference > 1e-9 {
		t.Errorf(
			"distance aller = %.12f, distance retour = %.12f",
			forward,
			backward,
		)
	}
}

func TestDistanceBetweenRejectsInvalidPositions(t *testing.T) {
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
			_, err := DistanceBetween(test.from, test.to)
			if err == nil {
				t.Fatal("DistanceBetween() doit refuser cette position")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

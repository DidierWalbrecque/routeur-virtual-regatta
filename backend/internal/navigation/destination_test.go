package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestDestinationPosition(t *testing.T) {
	const oneDegreeNauticalMiles = 60.04046073261873

	tests := []struct {
		name     string
		from     Position
		heading  Heading
		distance NauticalMiles
		want     Position
	}{
		{
			name:     "distance nulle",
			from:     Position{Latitude: 48.8566, Longitude: 2.3522},
			heading:  123,
			distance: 0,
			want:     Position{Latitude: 48.8566, Longitude: 2.3522},
		},
		{
			name:     "un degré vers le nord",
			from:     Position{},
			heading:  0,
			distance: oneDegreeNauticalMiles,
			want:     Position{Latitude: 1, Longitude: 0},
		},
		{
			name:     "un degré vers l'est",
			from:     Position{},
			heading:  90,
			distance: oneDegreeNauticalMiles,
			want:     Position{Latitude: 0, Longitude: 1},
		},
		{
			name:     "passage de l'antiméridien",
			from:     Position{Latitude: 0, Longitude: 179},
			heading:  90,
			distance: 2 * oneDegreeNauticalMiles,
			want:     Position{Latitude: 0, Longitude: -179},
		},
		{
			name:     "Paris vers New York",
			from:     Position{Latitude: 48.8566, Longitude: 2.3522},
			heading:  291.7938627483058,
			distance: 3151.858127424209,
			want:     Position{Latitude: 40.7128, Longitude: -74.0060},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DestinationPosition(
				test.from,
				test.heading,
				test.distance,
			)
			if err != nil {
				t.Fatalf(
					"DestinationPosition() a retourné une erreur inattendue: %v",
					err,
				)
			}

			if difference := math.Abs(
				float64(got.Latitude - test.want.Latitude),
			); difference > 1e-9 {
				t.Errorf(
					"latitude = %.12f, attendue %.12f",
					got.Latitude,
					test.want.Latitude,
				)
			}
			if difference := math.Abs(
				float64(got.Longitude - test.want.Longitude),
			); difference > 1e-9 {
				t.Errorf(
					"longitude = %.12f, attendue %.12f",
					got.Longitude,
					test.want.Longitude,
				)
			}
			if err := got.Validate(); err != nil {
				t.Errorf("la position retournée doit être valide: %v", err)
			}
		})
	}
}

func TestDestinationPositionPreservesTravelledDistance(t *testing.T) {
	from := Position{Latitude: -33.8688, Longitude: 151.2093}
	const (
		heading  Heading       = 42.5
		distance NauticalMiles = 750
	)

	to, err := DestinationPosition(from, heading, distance)
	if err != nil {
		t.Fatalf("DestinationPosition() a échoué: %v", err)
	}
	gotDistance, err := DistanceBetween(from, to)
	if err != nil {
		t.Fatalf("DistanceBetween() a échoué: %v", err)
	}

	if difference := math.Abs(
		float64(gotDistance - distance),
	); difference > 1e-9 {
		t.Errorf(
			"distance parcourue = %.12f, attendue %.12f",
			gotDistance,
			distance,
		)
	}
}

func TestDestinationPositionRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		from      Position
		heading   Heading
		distance  NauticalMiles
		errorPart string
	}{
		{
			name:      "position invalide",
			from:      Position{Latitude: 91},
			heading:   0,
			distance:  1,
			errorPart: "position de départ",
		},
		{
			name:      "cap invalide",
			from:      Position{},
			heading:   360,
			distance:  1,
			errorPart: "cap invalide",
		},
		{
			name:      "distance négative",
			from:      Position{},
			heading:   0,
			distance:  -1,
			errorPart: "distance invalide",
		},
		{
			name:      "distance non finie",
			from:      Position{},
			heading:   0,
			distance:  NauticalMiles(math.NaN()),
			errorPart: "distance invalide",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DestinationPosition(
				test.from,
				test.heading,
				test.distance,
			)
			if err == nil {
				t.Fatal("DestinationPosition() doit refuser ces entrées")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf(
					"erreur = %q, doit contenir %q",
					err,
					test.errorPart,
				)
			}
		})
	}
}

func TestNauticalMilesValidate(t *testing.T) {
	for _, valid := range []NauticalMiles{0, 1, 3151.858127424209} {
		if err := valid.Validate(); err != nil {
			t.Errorf("NauticalMiles(%v) doit être valide: %v", valid, err)
		}
	}

	for _, invalid := range []NauticalMiles{
		-0.1,
		NauticalMiles(math.NaN()),
		NauticalMiles(math.Inf(1)),
	} {
		if err := invalid.Validate(); err == nil {
			t.Errorf("NauticalMiles(%v) doit être invalide", invalid)
		}
	}
}

package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestNormalizeLongitude(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  Longitude
	}{
		{name: "zéro", value: 0, want: 0},
		{name: "limite ouest", value: -180, want: -180},
		{name: "limite est canonique", value: 180, want: -180},
		{name: "passage vers l'est", value: 181, want: -179},
		{name: "passage vers l'ouest", value: -181, want: 179},
		{name: "longitude NOAA", value: 359.75, want: -0.25},
		{name: "tour complet", value: 360, want: 0},
		{name: "plusieurs tours positifs", value: 721, want: 1},
		{name: "plusieurs tours négatifs", value: -721, want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeLongitude(test.value)
			if err != nil {
				t.Fatalf(
					"NormalizeLongitude() a retourné une erreur: %v",
					err,
				)
			}
			if got != test.want {
				t.Errorf(
					"NormalizeLongitude(%v) = %v, attendu %v",
					test.value,
					got,
					test.want,
				)
			}
		})
	}
}

func TestNormalizeLongitudeRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NormalizeLongitude(value); err == nil {
			t.Errorf("NormalizeLongitude(%v) doit retourner une erreur", value)
		}
	}
}

func TestShortestLongitudeDelta(t *testing.T) {
	tests := []struct {
		name string
		from Longitude
		to   Longitude
		want float64
	}{
		{name: "identiques", from: 42, to: 42, want: 0},
		{name: "antiméridien vers l'est", from: 179, to: -179, want: 2},
		{name: "antiméridien vers l'ouest", from: -179, to: 179, want: -2},
		{name: "vingt degrés vers l'est", from: 170, to: -170, want: 20},
		{name: "vingt degrés vers l'ouest", from: -170, to: 170, want: -20},
		{name: "méridiens équivalents", from: 180, to: -180, want: 0},
		{name: "opposé canonique", from: 0, to: 180, want: -180},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ShortestLongitudeDelta(test.from, test.to)
			if err != nil {
				t.Fatalf(
					"ShortestLongitudeDelta() a retourné une erreur: %v",
					err,
				)
			}
			if got != test.want {
				t.Errorf(
					"ShortestLongitudeDelta(%v, %v) = %v, attendu %v",
					test.from,
					test.to,
					got,
					test.want,
				)
			}
		})
	}
}

func TestShortestLongitudeDeltaRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		from      Longitude
		to        Longitude
		errorPart string
	}{
		{
			name:      "départ invalide",
			from:      181,
			to:        0,
			errorPart: "départ",
		},
		{
			name:      "arrivée invalide",
			from:      0,
			to:        -181,
			errorPart: "arrivée",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ShortestLongitudeDelta(test.from, test.to)
			if err == nil {
				t.Fatal("la longitude invalide doit être refusée")
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

func TestDestinationPositionCrossesAntimeridianBothWays(t *testing.T) {
	const twoDegrees = NauticalMiles(
		2 * earthRadiusNauticalMiles * math.Pi / 180,
	)

	tests := []struct {
		name    string
		from    Position
		heading Heading
		want    Longitude
	}{
		{
			name:    "vers l'est",
			from:    Position{Latitude: 0, Longitude: 179},
			heading: 90,
			want:    -179,
		},
		{
			name:    "vers l'ouest",
			from:    Position{Latitude: 0, Longitude: -179},
			heading: 270,
			want:    179,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DestinationPosition(
				test.from,
				test.heading,
				twoDegrees,
			)
			if err != nil {
				t.Fatalf(
					"DestinationPosition() a retourné une erreur: %v",
					err,
				)
			}
			if difference := math.Abs(
				float64(got.Longitude - test.want),
			); difference > 1e-9 {
				t.Errorf(
					"longitude = %.12f, attendue %.12f",
					got.Longitude,
					test.want,
				)
			}
		})
	}
}

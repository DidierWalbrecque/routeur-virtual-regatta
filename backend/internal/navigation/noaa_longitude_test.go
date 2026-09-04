package navigation

import (
	"math"
	"testing"
)

func TestLongitudeFromNOAA(t *testing.T) {
	tests := []struct {
		name string
		from NOAALongitude
		want Longitude
	}{
		{name: "méridien origine", from: 0, want: 0},
		{name: "est", from: 90, want: 90},
		{name: "antiméridien", from: 180, want: -180},
		{name: "ouest", from: 270, want: -90},
		{name: "pas NOAA près de Greenwich", from: 359.75, want: -0.25},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LongitudeFromNOAA(test.from)
			if err != nil {
				t.Fatalf(
					"LongitudeFromNOAA() a retourné une erreur: %v",
					err,
				)
			}
			if got != test.want {
				t.Errorf(
					"LongitudeFromNOAA(%v) = %v, attendu %v",
					test.from,
					got,
					test.want,
				)
			}
		})
	}
}

func TestNOAALongitudeFromLongitude(t *testing.T) {
	tests := []struct {
		name string
		from Longitude
		want NOAALongitude
	}{
		{name: "méridien origine", from: 0, want: 0},
		{name: "est", from: 90, want: 90},
		{name: "ouest", from: -90, want: 270},
		{name: "antiméridien ouest", from: -180, want: 180},
		{name: "antiméridien est", from: 180, want: 180},
		{name: "quart de degré ouest", from: -0.25, want: 359.75},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NOAALongitudeFromLongitude(test.from)
			if err != nil {
				t.Fatalf(
					"NOAALongitudeFromLongitude() a retourné une erreur: %v",
					err,
				)
			}
			if got != test.want {
				t.Errorf(
					"NOAALongitudeFromLongitude(%v) = %v, attendu %v",
					test.from,
					got,
					test.want,
				)
			}
		})
	}
}

func TestNOAALongitudeRoundTrip(t *testing.T) {
	for _, original := range []NOAALongitude{0, 90, 180, 270, 359.75} {
		longitude, err := LongitudeFromNOAA(original)
		if err != nil {
			t.Fatalf(
				"LongitudeFromNOAA(%v) a retourné une erreur: %v",
				original,
				err,
			)
		}

		got, err := NOAALongitudeFromLongitude(longitude)
		if err != nil {
			t.Fatalf(
				"NOAALongitudeFromLongitude(%v) a retourné une erreur: %v",
				longitude,
				err,
			)
		}
		if got != original {
			t.Errorf(
				"aller-retour NOAA %v = %v, attendu %v",
				original,
				got,
				original,
			)
		}
	}
}

func TestLongitudeFromNOAARejectsInvalidValues(t *testing.T) {
	values := []NOAALongitude{
		-0.25,
		360,
		NOAALongitude(math.NaN()),
		NOAALongitude(math.Inf(1)),
		NOAALongitude(math.Inf(-1)),
	}

	for _, value := range values {
		if _, err := LongitudeFromNOAA(value); err == nil {
			t.Errorf(
				"LongitudeFromNOAA(%v) doit retourner une erreur",
				value,
			)
		}
	}
}

func TestNOAALongitudeFromLongitudeRejectsInvalidValues(t *testing.T) {
	values := []Longitude{
		-181,
		181,
		Longitude(math.NaN()),
		Longitude(math.Inf(1)),
		Longitude(math.Inf(-1)),
	}

	for _, value := range values {
		if _, err := NOAALongitudeFromLongitude(value); err == nil {
			t.Errorf(
				"NOAALongitudeFromLongitude(%v) doit retourner une erreur",
				value,
			)
		}
	}
}

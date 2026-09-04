package main

import (
	"math"
	"strings"
	"testing"
)

func TestParseVirtualRegattaPosition(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		wantLatitude  float64
		wantLongitude float64
	}{
		{
			name:          "nord et est",
			value:         "13°N 04'08\" - 121°E 37'42\"",
			wantLatitude:  13.068888888888889,
			wantLongitude: 121.62833333333333,
		},
		{
			name:          "sud et ouest avec symboles typographiques",
			value:         "33°S 52′08″ — 151°W 12′33″",
			wantLatitude:  -33.86888888888889,
			wantLongitude: -151.20916666666668,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseVirtualRegattaPosition(test.value)
			if err != nil {
				t.Fatalf(
					"parseVirtualRegattaPosition() a échoué: %v",
					err,
				)
			}
			if difference := math.Abs(
				float64(got.Latitude) - test.wantLatitude,
			); difference > 1e-12 {
				t.Errorf(
					"latitude = %.12f, attendue %.12f",
					got.Latitude,
					test.wantLatitude,
				)
			}
			if difference := math.Abs(
				float64(got.Longitude) - test.wantLongitude,
			); difference > 1e-12 {
				t.Errorf(
					"longitude = %.12f, attendue %.12f",
					got.Longitude,
					test.wantLongitude,
				)
			}
		})
	}
}

func TestParseVirtualRegattaPositionRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{
		"13.068889, 121.628333",
		"13°N 60'00\" - 121°E 37'42\"",
		"91°N 00'00\" - 121°E 37'42\"",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := parseVirtualRegattaPosition(value)
			if err == nil {
				t.Fatal("la position doit être refusée")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("l'erreur doit être explicite")
			}
		})
	}
}

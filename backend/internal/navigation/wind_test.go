package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestWindFromComponents(t *testing.T) {
	tests := []struct {
		name          string
		u             WindComponent
		v             WindComponent
		wantSpeed     MetersPerSecond
		wantDirection WindDirection
	}{
		{
			name:          "vent du nord",
			u:             0,
			v:             -10,
			wantSpeed:     10,
			wantDirection: 0,
		},
		{
			name:          "vent de l'est",
			u:             -10,
			v:             0,
			wantSpeed:     10,
			wantDirection: 90,
		},
		{
			name:          "vent du sud",
			u:             0,
			v:             10,
			wantSpeed:     10,
			wantDirection: 180,
		},
		{
			name:          "vent de l'ouest",
			u:             10,
			v:             0,
			wantSpeed:     10,
			wantDirection: 270,
		},
		{
			name:          "vent du nord-est",
			u:             -3,
			v:             -4,
			wantSpeed:     5,
			wantDirection: 36.86989764584402,
		},
		{
			name:          "vent du sud-ouest",
			u:             3,
			v:             4,
			wantSpeed:     5,
			wantDirection: 216.86989764584402,
		},
		{
			name:          "vent nul",
			u:             0,
			v:             0,
			wantSpeed:     0,
			wantDirection: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := WindFromComponents(test.u, test.v)
			if err != nil {
				t.Fatalf(
					"WindFromComponents() a retourné une erreur: %v",
					err,
				)
			}

			if difference := math.Abs(
				float64(got.Speed - test.wantSpeed),
			); difference > 1e-12 {
				t.Errorf(
					"vitesse = %.12f m/s, attendue %.12f m/s",
					got.Speed,
					test.wantSpeed,
				)
			}
			if difference := math.Abs(
				float64(got.Direction - test.wantDirection),
			); difference > 1e-12 {
				t.Errorf(
					"direction = %.12f°, attendue %.12f°",
					got.Direction,
					test.wantDirection,
				)
			}
		})
	}
}

func TestWindFromComponentsRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		u         WindComponent
		v         WindComponent
		errorPart string
	}{
		{
			name:      "U non défini",
			u:         WindComponent(math.NaN()),
			v:         0,
			errorPart: "U",
		},
		{
			name:      "U infini",
			u:         WindComponent(math.Inf(1)),
			v:         0,
			errorPart: "U",
		},
		{
			name:      "V non défini",
			u:         0,
			v:         WindComponent(math.NaN()),
			errorPart: "V",
		},
		{
			name:      "V infini",
			u:         0,
			v:         WindComponent(math.Inf(-1)),
			errorPart: "V",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := WindFromComponents(test.u, test.v)
			if err == nil {
				t.Fatal("la composante invalide doit être refusée")
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

func TestWindValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		wind Wind
	}{
		{
			name: "vitesse négative",
			wind: Wind{Speed: -1, Direction: 0},
		},
		{
			name: "vitesse non définie",
			wind: Wind{
				Speed:     MetersPerSecond(math.NaN()),
				Direction: 0,
			},
		},
		{
			name: "direction négative",
			wind: Wind{Speed: 1, Direction: -1},
		},
		{
			name: "direction égale à 360",
			wind: Wind{Speed: 1, Direction: 360},
		},
		{
			name: "direction non définie",
			wind: Wind{
				Speed:     1,
				Direction: WindDirection(math.NaN()),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.wind.Validate(); err == nil {
				t.Fatal("le vent invalide doit être refusé")
			}
		})
	}
}

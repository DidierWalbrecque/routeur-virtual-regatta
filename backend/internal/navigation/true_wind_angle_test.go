package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestTrueWindAngleFromHeading(t *testing.T) {
	tests := []struct {
		name          string
		heading       Heading
		windDirection WindDirection
		want          TrueWindAngle
	}{
		{
			name:          "face au vent",
			heading:       0,
			windDirection: 0,
			want:          0,
		},
		{
			name:          "vent de travers tribord",
			heading:       0,
			windDirection: 90,
			want:          90,
		},
		{
			name:          "vent de travers bâbord",
			heading:       0,
			windDirection: 270,
			want:          90,
		},
		{
			name:          "vent arrière",
			heading:       0,
			windDirection: 180,
			want:          180,
		},
		{
			name:          "passage par zéro tribord",
			heading:       350,
			windDirection: 10,
			want:          20,
		},
		{
			name:          "passage par zéro bâbord",
			heading:       10,
			windDirection: 350,
			want:          20,
		},
		{
			name:          "observation VR bâbord",
			heading:       321,
			windDirection: 222,
			want:          99,
		},
		{
			name:          "même TWA à tribord",
			heading:       321,
			windDirection: 60,
			want:          99,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := TrueWindAngleFromHeading(
				test.heading,
				test.windDirection,
			)
			if err != nil {
				t.Fatalf(
					"TrueWindAngleFromHeading() a retourné une erreur: %v",
					err,
				)
			}
			if difference := math.Abs(
				float64(got - test.want),
			); difference > 1e-12 {
				t.Errorf(
					"TWA = %.12f°, attendu %.12f°",
					got,
					test.want,
				)
			}
		})
	}
}

func TestTrueWindAngleFromHeadingRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name          string
		heading       Heading
		windDirection WindDirection
		errorPart     string
	}{
		{
			name:          "cap négatif",
			heading:       -1,
			windDirection: 0,
			errorPart:     "cap",
		},
		{
			name:          "cap égal à 360",
			heading:       360,
			windDirection: 0,
			errorPart:     "cap",
		},
		{
			name:          "direction négative",
			heading:       0,
			windDirection: -1,
			errorPart:     "direction",
		},
		{
			name:          "direction égale à 360",
			heading:       0,
			windDirection: 360,
			errorPart:     "direction",
		},
		{
			name:          "cap non défini",
			heading:       Heading(math.NaN()),
			windDirection: 0,
			errorPart:     "cap",
		},
		{
			name:          "direction infinie",
			heading:       0,
			windDirection: WindDirection(math.Inf(1)),
			errorPart:     "direction",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := TrueWindAngleFromHeading(
				test.heading,
				test.windDirection,
			)
			if err == nil {
				t.Fatal("l'entrée invalide doit être refusée")
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

func TestTrueWindAngleValidateRejectsInvalidValues(t *testing.T) {
	values := []TrueWindAngle{
		-1,
		181,
		TrueWindAngle(math.NaN()),
		TrueWindAngle(math.Inf(1)),
		TrueWindAngle(math.Inf(-1)),
	}

	for _, value := range values {
		if err := value.Validate(); err == nil {
			t.Errorf("TrueWindAngle(%v) doit être refusé", value)
		}
	}
}

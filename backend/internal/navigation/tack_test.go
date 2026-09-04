package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestTackFromHeading(t *testing.T) {
	tests := []struct {
		name          string
		heading       Heading
		windDirection WindDirection
		want          Tack
	}{
		{
			name:          "observation VR bâbord",
			heading:       321,
			windDirection: 222,
			want:          TackPort,
		},
		{
			name:          "même TWA à tribord",
			heading:       321,
			windDirection: 60,
			want:          TackStarboard,
		},
		{
			name:          "passage par zéro bâbord",
			heading:       10,
			windDirection: 350,
			want:          TackPort,
		},
		{
			name:          "passage par zéro tribord",
			heading:       350,
			windDirection: 10,
			want:          TackStarboard,
		},
		{
			name:          "vent de face",
			heading:       0,
			windDirection: 0,
			want:          TackUndefined,
		},
		{
			name:          "vent arrière",
			heading:       0,
			windDirection: 180,
			want:          TackUndefined,
		},
		{
			name:          "vent arrière avec cap au sud",
			heading:       180,
			windDirection: 0,
			want:          TackUndefined,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := TackFromHeading(
				test.heading,
				test.windDirection,
			)
			if err != nil {
				t.Fatalf(
					"TackFromHeading() a retourné une erreur: %v",
					err,
				)
			}
			if got != test.want {
				t.Errorf(
					"amure = %q, attendue %q",
					got,
					test.want,
				)
			}
		})
	}
}

func TestTackFromHeadingRejectsInvalidValues(t *testing.T) {
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
			name:          "cap non défini",
			heading:       Heading(math.NaN()),
			windDirection: 0,
			errorPart:     "cap",
		},
		{
			name:          "direction égale à 360",
			heading:       0,
			windDirection: 360,
			errorPart:     "direction",
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
			_, err := TackFromHeading(
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

func TestTackValidate(t *testing.T) {
	for _, tack := range []Tack{
		TackUndefined,
		TackPort,
		TackStarboard,
	} {
		if err := tack.Validate(); err != nil {
			t.Errorf("Tack(%q) a retourné une erreur: %v", tack, err)
		}
	}

	if err := Tack("invalid").Validate(); err == nil {
		t.Fatal("une amure inconnue doit être refusée")
	}
}

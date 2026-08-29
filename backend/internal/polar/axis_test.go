package polar

import (
	"math"
	"strings"
	"testing"
)

func TestFindAxisBounds(t *testing.T) {
	axis := []WindSpeed{0, 10, 20, 30}

	tests := []struct {
		name  string
		value WindSpeed
		want  axisBounds
	}{
		{
			name:  "première valeur exacte",
			value: 0,
			want:  axisBounds{LowerIndex: 0, UpperIndex: 0},
		},
		{
			name:  "valeur intérieure exacte",
			value: 20,
			want:  axisBounds{LowerIndex: 2, UpperIndex: 2},
		},
		{
			name:  "valeur entre deux points",
			value: 15,
			want:  axisBounds{LowerIndex: 1, UpperIndex: 2},
		},
		{
			name:  "dernière valeur exacte",
			value: 30,
			want:  axisBounds{LowerIndex: 3, UpperIndex: 3},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := findAxisBounds(axis, test.value)
			if err != nil {
				t.Fatalf("findAxisBounds() a retourné une erreur inattendue: %v", err)
			}
			if got != test.want {
				t.Errorf("findAxisBounds() = %+v, attendu %+v", got, test.want)
			}
		})
	}
}

func TestFindAxisBoundsSupportsWindAngles(t *testing.T) {
	axis := []TrueWindAngle{0, 45, 90, 135, 180}

	got, err := findAxisBounds(axis, TrueWindAngle(60))
	if err != nil {
		t.Fatalf("findAxisBounds() a retourné une erreur inattendue: %v", err)
	}

	want := axisBounds{LowerIndex: 1, UpperIndex: 2}
	if got != want {
		t.Errorf("findAxisBounds() = %+v, attendu %+v", got, want)
	}
}

func TestFindAxisBoundsRejectsInvalidValues(t *testing.T) {
	axis := []WindSpeed{0, 10, 20}

	tests := []struct {
		name      string
		axis      []WindSpeed
		value     WindSpeed
		errorPart string
	}{
		{
			name:      "axe vide",
			axis:      nil,
			value:     10,
			errorPart: "axe vide",
		},
		{
			name:      "valeur inférieure à l'axe",
			axis:      axis,
			value:     -1,
			errorPart: "hors des limites",
		},
		{
			name:      "valeur supérieure à l'axe",
			axis:      axis,
			value:     21,
			errorPart: "hors des limites",
		},
		{
			name:      "valeur non finie",
			axis:      axis,
			value:     WindSpeed(math.NaN()),
			errorPart: "doit être finie",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := findAxisBounds(test.axis, test.value)
			if err == nil {
				t.Fatal("findAxisBounds() devait retourner une erreur")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

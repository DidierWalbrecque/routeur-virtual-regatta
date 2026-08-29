package polar

import (
	"math"
	"strings"
	"testing"
)

func TestInterpolateWindSpeed(t *testing.T) {
	windSpeeds := []WindSpeed{16, 18, 30}
	boatSpeeds := []BoatSpeed{29.6, 31.8, 20}

	tests := []struct {
		name   string
		target WindSpeed
		want   BoatSpeed
	}{
		{
			name:   "première valeur exacte",
			target: 16,
			want:   29.6,
		},
		{
			name:   "valeur intermédiaire",
			target: 17,
			want:   30.7,
		},
		{
			name:   "valeur intérieure exacte",
			target: 18,
			want:   31.8,
		},
		{
			name:   "intervalle avec vitesse décroissante",
			target: 24,
			want:   25.9,
		},
		{
			name:   "dernière valeur exacte",
			target: 30,
			want:   20,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := interpolateWindSpeed(windSpeeds, boatSpeeds, test.target)
			if err != nil {
				t.Fatalf("interpolateWindSpeed() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("interpolateWindSpeed() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestInterpolateWindSpeedRejectsDifferentDimensions(t *testing.T) {
	_, err := interpolateWindSpeed(
		[]WindSpeed{0, 10, 20},
		[]BoatSpeed{0, 12},
		10,
	)
	if err == nil {
		t.Fatal("interpolateWindSpeed() doit refuser des axes de dimensions différentes")
	}
	if !strings.Contains(err.Error(), "dimensions incompatibles") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestInterpolateWindSpeedRejectsTargetOutsideAxis(t *testing.T) {
	_, err := interpolateWindSpeed(
		[]WindSpeed{0, 10, 20},
		[]BoatSpeed{0, 12, 18},
		21,
	)
	if err == nil {
		t.Fatal("interpolateWindSpeed() doit refuser une TWS hors de l'axe")
	}
	if !strings.Contains(err.Error(), "hors des limites") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

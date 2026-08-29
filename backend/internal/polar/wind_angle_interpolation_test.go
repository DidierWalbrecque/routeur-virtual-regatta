package polar

import (
	"math"
	"strings"
	"testing"
)

func TestInterpolateWindAngle(t *testing.T) {
	windAngles := []TrueWindAngle{80, 90, 100}
	boatSpeeds := []BoatSpeed{30.7, 33.65, 32}

	tests := []struct {
		name   string
		target TrueWindAngle
		want   BoatSpeed
	}{
		{
			name:   "première valeur exacte",
			target: 80,
			want:   30.7,
		},
		{
			name:   "intervalle avec vitesse croissante",
			target: 85,
			want:   32.175,
		},
		{
			name:   "valeur intérieure exacte",
			target: 90,
			want:   33.65,
		},
		{
			name:   "intervalle avec vitesse décroissante",
			target: 95,
			want:   32.825,
		},
		{
			name:   "dernière valeur exacte",
			target: 100,
			want:   32,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := interpolateWindAngle(windAngles, boatSpeeds, test.target)
			if err != nil {
				t.Fatalf("interpolateWindAngle() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("interpolateWindAngle() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestInterpolateWindAngleRejectsDifferentDimensions(t *testing.T) {
	_, err := interpolateWindAngle(
		[]TrueWindAngle{80, 90, 100},
		[]BoatSpeed{30, 32},
		85,
	)
	if err == nil {
		t.Fatal("interpolateWindAngle() doit refuser des axes de dimensions différentes")
	}
	if !strings.Contains(err.Error(), "dimensions incompatibles") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestInterpolateWindAngleRejectsTargetOutsideAxis(t *testing.T) {
	_, err := interpolateWindAngle(
		[]TrueWindAngle{80, 90, 100},
		[]BoatSpeed{30, 32, 31},
		79,
	)
	if err == nil {
		t.Fatal("interpolateWindAngle() doit refuser une TWA hors de l'axe")
	}
	if !strings.Contains(err.Error(), "hors des limites") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

package polar

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpeedAt(t *testing.T) {
	polar := speedAtTestPolar()

	got, err := polar.SpeedAt(5, 17, 85)
	if err != nil {
		t.Fatalf("SpeedAt() a retourné une erreur inattendue: %v", err)
	}

	const want BoatSpeed = 32.175
	if difference := math.Abs(float64(got - want)); difference > 1e-9 {
		t.Errorf("SpeedAt() = %.12f, attendu %.12f", got, want)
	}
}

func TestSpeedAtWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	tests := []struct {
		name      string
		windSpeed WindSpeed
		windAngle TrueWindAngle
		want      BoatSpeed
	}{
		{
			name:      "point exact Code0",
			windSpeed: 20,
			windAngle: 90,
			want:      36.8,
		},
		{
			name:      "interpolation bilinéaire Code0",
			windSpeed: 17,
			windAngle: 85,
			want:      32.175,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, speedErr := polar.SpeedAt(5, test.windSpeed, test.windAngle)
			if speedErr != nil {
				t.Fatalf("SpeedAt() a retourné une erreur inattendue: %v", speedErr)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("SpeedAt() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestSpeedAtRejectsNilPolar(t *testing.T) {
	var polar *Polar

	_, err := polar.SpeedAt(5, 17, 85)
	if err == nil {
		t.Fatal("SpeedAt() doit refuser une polaire nil")
	}
	if !strings.Contains(err.Error(), "polaire est nil") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestSpeedAtRejectsUnknownSail(t *testing.T) {
	polar := speedAtTestPolar()

	_, err := polar.SpeedAt(99, 17, 85)
	if err == nil {
		t.Fatal("SpeedAt() doit refuser une voile inconnue")
	}
	if !strings.Contains(err.Error(), "voile 99 introuvable") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestSpeedAtWrapsInterpolationError(t *testing.T) {
	polar := speedAtTestPolar()

	_, err := polar.SpeedAt(5, 15, 85)
	if err == nil {
		t.Fatal("SpeedAt() doit refuser une TWS hors de la grille")
	}
	if !strings.Contains(err.Error(), "voile \"Code0\" (5)") {
		t.Errorf("l'erreur doit identifier la voile: %v", err)
	}
	if !strings.Contains(err.Error(), "hors des limites") {
		t.Errorf("l'erreur doit conserver la cause de l'interpolation: %v", err)
	}
}

func speedAtTestPolar() *Polar {
	return &Polar{
		WindSpeeds: []WindSpeed{16, 18},
		WindAngles: []TrueWindAngle{80, 90},
		Sails: []Sail{
			{
				ID:   1,
				Name: "Jib",
				Speeds: SpeedGrid{
					Values: [][]BoatSpeed{
						{20, 22},
						{21, 23},
					},
				},
			},
			{
				ID:   5,
				Name: "Code0",
				Speeds: SpeedGrid{
					Values: [][]BoatSpeed{
						{29.6, 31.8},
						{32.6, 34.7},
					},
				},
			},
		},
	}
}

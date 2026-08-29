package polar

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestEffectiveSpeedAt(t *testing.T) {
	tests := []struct {
		name      string
		options   SpeedOptions
		hullRatio SpeedRatio
		want      BoatSpeed
	}{
		{
			name:      "coefficient global uniquement",
			options:   SpeedOptions{},
			hullRatio: 1.003,
			want:      33.78375,
		},
		{
			name:      "coefficient global et polish",
			options:   SpeedOptions{HullPolish: true},
			hullRatio: 1.003,
			want:      33.88510125,
		},
		{
			name:      "coefficient de polish absent mais désactivé",
			options:   SpeedOptions{},
			hullRatio: 0,
			want:      33.78375,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			polar := speedAtTestPolar()
			polar.GlobalSpeedRatio = 1.05
			polar.Hull.SpeedRatio = test.hullRatio

			got, err := polar.EffectiveSpeedAt(5, 17, 85, test.options)
			if err != nil {
				t.Fatalf("EffectiveSpeedAt() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("EffectiveSpeedAt() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestEffectiveSpeedAtDoesNotChangeRawSpeed(t *testing.T) {
	polar := speedAtTestPolar()
	polar.GlobalSpeedRatio = 1.05
	polar.Hull.SpeedRatio = 1.003

	rawSpeed, err := polar.SpeedAt(5, 17, 85)
	if err != nil {
		t.Fatalf("SpeedAt() a retourné une erreur inattendue: %v", err)
	}

	const want BoatSpeed = 32.175
	if difference := math.Abs(float64(rawSpeed - want)); difference > 1e-9 {
		t.Errorf("SpeedAt() = %.12f, attendu %.12f", rawSpeed, want)
	}
}

func TestEffectiveSpeedAtWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	got, err := polar.EffectiveSpeedAt(
		5,
		20,
		90,
		SpeedOptions{HullPolish: true},
	)
	if err != nil {
		t.Fatalf("EffectiveSpeedAt() a retourné une erreur inattendue: %v", err)
	}

	const want BoatSpeed = 36.9104
	if difference := math.Abs(float64(got - want)); difference > 1e-9 {
		t.Errorf("EffectiveSpeedAt() = %.12f, attendu %.12f", got, want)
	}
}

func TestEffectiveSpeedAtRejectsInvalidCoefficients(t *testing.T) {
	tests := []struct {
		name        string
		globalRatio SpeedRatio
		hullRatio   SpeedRatio
		options     SpeedOptions
		errorPart   string
	}{
		{
			name:        "coefficient global nul",
			globalRatio: 0,
			hullRatio:   1.003,
			options:     SpeedOptions{},
			errorPart:   "globalSpeedRatio",
		},
		{
			name:        "coefficient global négatif",
			globalRatio: -1,
			hullRatio:   1.003,
			options:     SpeedOptions{},
			errorPart:   "globalSpeedRatio",
		},
		{
			name:        "coefficient global non fini",
			globalRatio: SpeedRatio(math.NaN()),
			hullRatio:   1.003,
			options:     SpeedOptions{},
			errorPart:   "globalSpeedRatio",
		},
		{
			name:        "coefficient de polish nul",
			globalRatio: 1,
			hullRatio:   0,
			options:     SpeedOptions{HullPolish: true},
			errorPart:   "hull.speedRatio",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			polar := speedAtTestPolar()
			polar.GlobalSpeedRatio = test.globalRatio
			polar.Hull.SpeedRatio = test.hullRatio

			_, err := polar.EffectiveSpeedAt(5, 17, 85, test.options)
			if err == nil {
				t.Fatal("EffectiveSpeedAt() doit refuser ce coefficient")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestEffectiveSpeedAtPreservesSpeedErrors(t *testing.T) {
	tests := []struct {
		name      string
		polar     *Polar
		sailID    SailID
		windSpeed WindSpeed
		errorPart string
	}{
		{
			name:      "polaire nil",
			polar:     nil,
			sailID:    5,
			windSpeed: 17,
			errorPart: "polaire est nil",
		},
		{
			name:      "voile inconnue",
			polar:     speedAtTestPolar(),
			sailID:    99,
			windSpeed: 17,
			errorPart: "voile 99 introuvable",
		},
		{
			name:      "vent hors grille",
			polar:     speedAtTestPolar(),
			sailID:    5,
			windSpeed: 15,
			errorPart: "hors des limites",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.polar != nil {
				test.polar.GlobalSpeedRatio = 1
			}

			_, err := test.polar.EffectiveSpeedAt(
				test.sailID,
				test.windSpeed,
				85,
				SpeedOptions{},
			)
			if err == nil {
				t.Fatal("EffectiveSpeedAt() doit conserver l'erreur de vitesse brute")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

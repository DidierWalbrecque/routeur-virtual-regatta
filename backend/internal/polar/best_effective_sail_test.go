package polar

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestBestEffectiveSailAt(t *testing.T) {
	polar := bestSailTestPolar()
	polar.GlobalSpeedRatio = 1.05
	polar.Foil = ultimFoilTestProfile()
	polar.Hull.SpeedRatio = 1.003

	got, err := polar.BestEffectiveSailAt(
		17,
		85,
		[]SailID{1, 5},
		SpeedOptions{
			Foils:      true,
			HullPolish: true,
		},
	)
	if err != nil {
		t.Fatalf("BestEffectiveSailAt() a retourné une erreur inattendue: %v", err)
	}

	if got.SailID != 5 || got.SailName != "Code0" {
		t.Errorf(
			"voile = %q (%d), attendu %q (%d)",
			got.SailName,
			got.SailID,
			"Code0",
			5,
		)
	}

	const want BoatSpeed = 35.2405053
	if difference := math.Abs(float64(got.Speed - want)); difference > 1e-9 {
		t.Errorf("Speed = %.12f, attendu %.12f", got.Speed, want)
	}
}

func TestBestEffectiveSailAtWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	got, err := polar.BestEffectiveSailAt(
		20,
		90,
		[]SailID{1, 2, 5},
		SpeedOptions{
			Foils:      true,
			HullPolish: true,
		},
	)
	if err != nil {
		t.Fatalf("BestEffectiveSailAt() a retourné une erreur inattendue: %v", err)
	}

	if got.SailID != 5 || got.SailName != "Code0" {
		t.Errorf(
			"voile = %q (%d), attendu %q (%d)",
			got.SailName,
			got.SailID,
			"Code0",
			5,
		)
	}

	const want BoatSpeed = 38.386816
	if difference := math.Abs(float64(got.Speed - want)); difference > 1e-9 {
		t.Errorf("Speed = %.12f, attendu %.12f", got.Speed, want)
	}
}

func TestBestEffectiveSailAtRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name           string
		polar          *Polar
		availableSails []SailID
		errorPart      string
	}{
		{
			name:           "polaire nil",
			polar:          nil,
			availableSails: []SailID{5},
			errorPart:      "polaire est nil",
		},
		{
			name:           "aucune voile disponible",
			polar:          speedAtTestPolar(),
			availableSails: nil,
			errorPart:      "aucune voile disponible",
		},
		{
			name:           "coefficient global invalide",
			polar:          speedAtTestPolar(),
			availableSails: []SailID{5},
			errorPart:      "globalSpeedRatio",
		},
	}

	tests[1].polar.GlobalSpeedRatio = 1

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.polar.BestEffectiveSailAt(
				17,
				85,
				test.availableSails,
				SpeedOptions{},
			)
			if err == nil {
				t.Fatal("BestEffectiveSailAt() doit refuser cette entrée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

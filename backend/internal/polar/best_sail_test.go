package polar

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestBestSailAt(t *testing.T) {
	polar := bestSailTestPolar()

	tests := []struct {
		name           string
		availableSails []SailID
		wantID         SailID
		wantName       string
		wantSpeed      BoatSpeed
	}{
		{
			name:           "sélectionne la voile la plus rapide",
			availableSails: []SailID{1, 5, 7},
			wantID:         5,
			wantName:       "Code0",
			wantSpeed:      32.175,
		},
		{
			name:           "respecte les voiles disponibles",
			availableSails: []SailID{1},
			wantID:         1,
			wantName:       "Jib",
			wantSpeed:      21.5,
		},
		{
			name:           "conserve la première voile en cas d'égalité",
			availableSails: []SailID{7, 5},
			wantID:         7,
			wantName:       "LightGnk",
			wantSpeed:      32.175,
		},
		{
			name:           "ignore les identifiants dupliqués",
			availableSails: []SailID{1, 5, 5},
			wantID:         5,
			wantName:       "Code0",
			wantSpeed:      32.175,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := polar.BestSailAt(17, 85, test.availableSails)
			if err != nil {
				t.Fatalf("BestSailAt() a retourné une erreur inattendue: %v", err)
			}
			if got.SailID != test.wantID {
				t.Errorf("SailID = %d, attendu %d", got.SailID, test.wantID)
			}
			if got.SailName != test.wantName {
				t.Errorf("SailName = %q, attendu %q", got.SailName, test.wantName)
			}
			if difference := math.Abs(float64(got.Speed - test.wantSpeed)); difference > 1e-9 {
				t.Errorf("Speed = %.12f, attendu %.12f", got.Speed, test.wantSpeed)
			}
		})
	}
}

func TestBestSailAtWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	tests := []struct {
		name           string
		availableSails []SailID
		wantID         SailID
		wantName       string
		wantSpeed      BoatSpeed
	}{
		{
			name:           "voiles de base uniquement",
			availableSails: []SailID{1, 2},
			wantID:         1,
			wantName:       "Jib",
			wantSpeed:      34.96,
		},
		{
			name:           "option reaching disponible",
			availableSails: []SailID{1, 2, 5},
			wantID:         5,
			wantName:       "Code0",
			wantSpeed:      36.8,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, bestErr := polar.BestSailAt(20, 90, test.availableSails)
			if bestErr != nil {
				t.Fatalf("BestSailAt() a retourné une erreur inattendue: %v", bestErr)
			}
			if got.SailID != test.wantID || got.SailName != test.wantName {
				t.Errorf(
					"voile = %q (%d), attendu %q (%d)",
					got.SailName,
					got.SailID,
					test.wantName,
					test.wantID,
				)
			}
			if difference := math.Abs(float64(got.Speed - test.wantSpeed)); difference > 1e-9 {
				t.Errorf("Speed = %.12f, attendu %.12f", got.Speed, test.wantSpeed)
			}
		})
	}
}

func TestBestSailAtRejectsInvalidInputs(t *testing.T) {
	validPolar := bestSailTestPolar()

	tests := []struct {
		name           string
		polar          *Polar
		windSpeed      WindSpeed
		windAngle      TrueWindAngle
		availableSails []SailID
		errorPart      string
	}{
		{
			name:           "polaire nil",
			polar:          nil,
			windSpeed:      17,
			windAngle:      85,
			availableSails: []SailID{1},
			errorPart:      "polaire est nil",
		},
		{
			name:           "aucune voile disponible",
			polar:          validPolar,
			windSpeed:      17,
			windAngle:      85,
			availableSails: nil,
			errorPart:      "aucune voile disponible",
		},
		{
			name:           "identifiant inconnu",
			polar:          validPolar,
			windSpeed:      17,
			windAngle:      85,
			availableSails: []SailID{1, 99},
			errorPart:      "voile disponible 99 introuvable",
		},
		{
			name:           "vent hors de la grille",
			polar:          validPolar,
			windSpeed:      15,
			windAngle:      85,
			availableSails: []SailID{1},
			errorPart:      "hors des limites",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.polar.BestSailAt(
				test.windSpeed,
				test.windAngle,
				test.availableSails,
			)
			if err == nil {
				t.Fatal("BestSailAt() doit refuser cette entrée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func bestSailTestPolar() *Polar {
	code0Speeds := SpeedGrid{
		Values: [][]BoatSpeed{
			{29.6, 31.8},
			{32.6, 34.7},
		},
	}

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
				ID:     5,
				Name:   "Code0",
				Speeds: code0Speeds,
			},
			{
				ID:     7,
				Name:   "LightGnk",
				Speeds: code0Speeds,
			},
		},
	}
}

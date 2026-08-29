package polar

import (
	"math"
	"strings"
	"testing"
)

func TestInterpolateSpeedGrid(t *testing.T) {
	windSpeeds := []WindSpeed{16, 18}
	windAngles := []TrueWindAngle{80, 90}
	grid := SpeedGrid{
		Values: [][]BoatSpeed{
			{29.6, 31.8},
			{32.5, 34.8},
		},
	}

	tests := []struct {
		name            string
		targetWindSpeed WindSpeed
		targetWindAngle TrueWindAngle
		want            BoatSpeed
	}{
		{
			name:            "point exact de la grille",
			targetWindSpeed: 16,
			targetWindAngle: 80,
			want:            29.6,
		},
		{
			name:            "interpolation sur TWS uniquement",
			targetWindSpeed: 17,
			targetWindAngle: 80,
			want:            30.7,
		},
		{
			name:            "interpolation sur TWA uniquement",
			targetWindSpeed: 16,
			targetWindAngle: 85,
			want:            31.05,
		},
		{
			name:            "interpolation bilinéaire",
			targetWindSpeed: 17,
			targetWindAngle: 85,
			want:            32.175,
		},
		{
			name:            "dernier point exact de la grille",
			targetWindSpeed: 18,
			targetWindAngle: 90,
			want:            34.8,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := interpolateSpeedGrid(
				windSpeeds,
				windAngles,
				grid,
				test.targetWindSpeed,
				test.targetWindAngle,
			)
			if err != nil {
				t.Fatalf("interpolateSpeedGrid() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("interpolateSpeedGrid() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestInterpolateSpeedGridRejectsInvalidInputs(t *testing.T) {
	windSpeeds := []WindSpeed{16, 18}
	windAngles := []TrueWindAngle{80, 90}
	validGrid := SpeedGrid{
		Values: [][]BoatSpeed{
			{29.6, 31.8},
			{32.5, 34.8},
		},
	}

	tests := []struct {
		name            string
		grid            SpeedGrid
		targetWindSpeed WindSpeed
		targetWindAngle TrueWindAngle
		errorPart       string
	}{
		{
			name: "nombre de lignes incorrect",
			grid: SpeedGrid{
				Values: [][]BoatSpeed{{29.6, 31.8}},
			},
			targetWindSpeed: 17,
			targetWindAngle: 85,
			errorPart:       "1 lignes et 2 angles TWA",
		},
		{
			name: "nombre de colonnes incorrect",
			grid: SpeedGrid{
				Values: [][]BoatSpeed{
					{29.6, 31.8},
					{32.5},
				},
			},
			targetWindSpeed: 17,
			targetWindAngle: 85,
			errorPart:       "ligne 1 possède 1 colonnes",
		},
		{
			name:            "TWS hors de la grille",
			grid:            validGrid,
			targetWindSpeed: 15,
			targetWindAngle: 85,
			errorPart:       "hors des limites",
		},
		{
			name:            "TWA hors de la grille",
			grid:            validGrid,
			targetWindSpeed: 17,
			targetWindAngle: 79,
			errorPart:       "axe TWA",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := interpolateSpeedGrid(
				windSpeeds,
				windAngles,
				test.grid,
				test.targetWindSpeed,
				test.targetWindAngle,
			)
			if err == nil {
				t.Fatal("interpolateSpeedGrid() doit refuser cette entrée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

package weather

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassifyTenMeterWind(t *testing.T) {
	tests := []struct {
		name         string
		discipline   int
		category     int
		parameter    int
		surfaceType  int
		surfaceValue float64
		want         windComponent
	}{
		{
			name:         "composante U",
			discipline:   meteorologicalDiscipline,
			category:     momentumCategory,
			parameter:    uComponentParameter,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: tenMeters,
			want:         windComponentU,
		},
		{
			name:         "composante V",
			discipline:   meteorologicalDiscipline,
			category:     momentumCategory,
			parameter:    vComponentParameter,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: tenMeters,
			want:         windComponentV,
		},
		{
			name:         "autre discipline",
			discipline:   1,
			category:     momentumCategory,
			parameter:    uComponentParameter,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: tenMeters,
			want:         windComponentUnknown,
		},
		{
			name:         "autre catégorie",
			discipline:   meteorologicalDiscipline,
			category:     1,
			parameter:    uComponentParameter,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: tenMeters,
			want:         windComponentUnknown,
		},
		{
			name:         "autre paramètre",
			discipline:   meteorologicalDiscipline,
			category:     momentumCategory,
			parameter:    4,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: tenMeters,
			want:         windComponentUnknown,
		},
		{
			name:         "autre surface",
			discipline:   meteorologicalDiscipline,
			category:     momentumCategory,
			parameter:    uComponentParameter,
			surfaceType:  100,
			surfaceValue: tenMeters,
			want:         windComponentUnknown,
		},
		{
			name:         "autre hauteur",
			discipline:   meteorologicalDiscipline,
			category:     momentumCategory,
			parameter:    uComponentParameter,
			surfaceType:  heightAboveGroundSurface,
			surfaceValue: 80,
			want:         windComponentUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifyTenMeterWind(
				test.discipline,
				test.category,
				test.parameter,
				test.surfaceType,
				test.surfaceValue,
			)
			if got != test.want {
				t.Fatalf("composante = %v, attendu %v", got, test.want)
			}
		})
	}
}

func TestGRIB2WindGridValidate(t *testing.T) {
	validGrid := func() GRIB2WindGrid {
		return GRIB2WindGrid{
			ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
			Width:      2,
			Height:     1,
			Latitudes:  []float64{13, 13},
			Longitudes: []float64{121.5, 121.75},
			U:          []float64{-2, -2.5},
			V:          []float64{1, 1.5},
		}
	}

	if err := validGrid().Validate(); err != nil {
		t.Fatalf("grille valide refusée: %v", err)
	}

	tests := []struct {
		name    string
		change  func(*GRIB2WindGrid)
		wantErr string
	}{
		{
			name: "échéance nulle",
			change: func(grid *GRIB2WindGrid) {
				grid.ValidAt = time.Time{}
			},
			wantErr: "échéance",
		},
		{
			name: "dimensions invalides",
			change: func(grid *GRIB2WindGrid) {
				grid.Width = 0
			},
			wantErr: "dimensions",
		},
		{
			name: "composante U incomplète",
			change: func(grid *GRIB2WindGrid) {
				grid.U = grid.U[:1]
			},
			wantErr: "composante U",
		},
		{
			name: "latitude hors limites",
			change: func(grid *GRIB2WindGrid) {
				grid.Latitudes[0] = 91
			},
			wantErr: "latitude",
		},
		{
			name: "longitude hors limites",
			change: func(grid *GRIB2WindGrid) {
				grid.Longitudes[0] = 360
			},
			wantErr: "longitude",
		},
		{
			name: "composante U non finie",
			change: func(grid *GRIB2WindGrid) {
				grid.U[0] = math.NaN()
			},
			wantErr: "composante U",
		},
		{
			name: "composante V non finie",
			change: func(grid *GRIB2WindGrid) {
				grid.V[0] = math.Inf(1)
			},
			wantErr: "composante V",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			grid := validGrid()
			test.change(&grid)

			err := grid.Validate()
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %q, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadGRIB2WindGridRejectsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.grb2")

	_, err := LoadGRIB2WindGrid(path)
	if err == nil {
		t.Fatal("erreur attendue, aucune reçue")
	}
	if !strings.Contains(err.Error(), "ouverture du fichier GRIB2") {
		t.Fatalf("erreur inattendue: %v", err)
	}
}

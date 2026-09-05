package weather

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestGRIB2WindGridComponentsAt(t *testing.T) {
	grid := spatialTestGrid()

	tests := []struct {
		name     string
		position navigation.Position
		wantU    float64
		wantV    float64
	}{
		{
			name: "point exact",
			position: navigation.Position{
				Latitude:  10,
				Longitude: 120,
			},
			wantU: 0,
			wantV: 100,
		},
		{
			name: "milieu de la cellule",
			position: navigation.Position{
				Latitude:  5,
				Longitude: 125,
			},
			wantU: 15,
			wantV: 115,
		},
		{
			name: "interpolation en latitude seulement",
			position: navigation.Position{
				Latitude:  2.5,
				Longitude: 130,
			},
			wantU: 25,
			wantV: 125,
		},
		{
			name: "interpolation en longitude seulement",
			position: navigation.Position{
				Latitude:  10,
				Longitude: 122.5,
			},
			wantU: 2.5,
			wantV: 102.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u, v, err := grid.ComponentsAt(test.position)
			if err != nil {
				t.Fatalf("interpolation refusée: %v", err)
			}
			assertClose(t, "U", float64(u), test.wantU)
			assertClose(t, "V", float64(v), test.wantV)
		})
	}
}

func TestGRIB2WindGridComponentsAtConvertsWestLongitudeToNOAA(t *testing.T) {
	grid := GRIB2WindGrid{
		ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		Width:      2,
		Height:     1,
		Latitudes:  []float64{13, 13},
		Longitudes: []float64{230, 240},
		U:          []float64{10, 20},
		V:          []float64{-10, -20},
	}

	u, v, err := grid.ComponentsAt(navigation.Position{
		Latitude:  13,
		Longitude: -125,
	})
	if err != nil {
		t.Fatalf("interpolation refusée: %v", err)
	}

	assertClose(t, "U", float64(u), 15)
	assertClose(t, "V", float64(v), -15)
}

func TestGRIB2WindGridComponentsAtUnwrapsLongitudeDiscontinuity(t *testing.T) {
	grid := GRIB2WindGrid{
		ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		Width:      4,
		Height:     1,
		Latitudes:  []float64{0, 0, 0, 0},
		Longitudes: []float64{0, 90, -180, -90},
		U:          []float64{0, 1, 2, 3},
		V:          []float64{10, 11, 12, 13},
	}

	u, v, err := grid.ComponentsAt(navigation.Position{
		Latitude:  0,
		Longitude: -135,
	})
	if err != nil {
		t.Fatalf("interpolation après la coupure refusée: %v", err)
	}

	assertClose(t, "U", float64(u), 2.5)
	assertClose(t, "V", float64(v), 12.5)
}

func TestGRIB2WindGridComponentsAtWrapsGlobalLongitude(t *testing.T) {
	grid := GRIB2WindGrid{
		ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		Width:      4,
		Height:     1,
		Latitudes:  []float64{0, 0, 0, 0},
		Longitudes: []float64{0, 90, 180, 270},
		U:          []float64{0, 0, 0, 4},
		V:          []float64{2, 2, 2, 6},
	}

	u, v, err := grid.ComponentsAt(navigation.Position{
		Latitude:  0,
		Longitude: -45,
	})
	if err != nil {
		t.Fatalf("interpolation cyclique refusée: %v", err)
	}

	assertClose(t, "U", float64(u), 2)
	assertClose(t, "V", float64(v), 4)
}

func TestGRIB2WindGridWindAtPosition(t *testing.T) {
	grid := GRIB2WindGrid{
		ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		Width:      2,
		Height:     2,
		Latitudes:  []float64{10, 10, 0, 0},
		Longitudes: []float64{120, 130, 120, 130},
		U:          []float64{-3, -3, -3, -3},
		V:          []float64{-4, -4, -4, -4},
	}

	wind, err := grid.WindAtPosition(navigation.Position{
		Latitude:  5,
		Longitude: 125,
	})
	if err != nil {
		t.Fatalf("calcul du vent refusé: %v", err)
	}

	assertClose(t, "vitesse", float64(wind.Speed), 5)
	assertClose(t, "direction", float64(wind.Direction), 36.86989764584402)
}

func TestGRIB2WindGridComponentsAtRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name     string
		grid     GRIB2WindGrid
		position navigation.Position
		wantErr  string
	}{
		{
			name: "latitude hors de la grille",
			grid: spatialTestGrid(),
			position: navigation.Position{
				Latitude:  20,
				Longitude: 125,
			},
			wantErr: "latitude",
		},
		{
			name: "longitude hors d'une grille régionale",
			grid: spatialTestGrid(),
			position: navigation.Position{
				Latitude:  5,
				Longitude: 140,
			},
			wantErr: "longitude",
		},
		{
			name: "position invalide",
			grid: spatialTestGrid(),
			position: navigation.Position{
				Latitude:  navigation.Latitude(math.NaN()),
				Longitude: 125,
			},
			wantErr: "position invalide",
		},
		{
			name: "tableaux incompatibles",
			grid: func() GRIB2WindGrid {
				grid := spatialTestGrid()
				grid.U = grid.U[:3]
				return grid
			}(),
			position: navigation.Position{
				Latitude:  5,
				Longitude: 125,
			},
			wantErr: "incompatibles",
		},
		{
			name: "latitudes non décroissantes",
			grid: func() GRIB2WindGrid {
				grid := spatialTestGrid()
				grid.Latitudes[2] = 20
				grid.Latitudes[3] = 20
				return grid
			}(),
			position: navigation.Position{
				Latitude:  5,
				Longitude: 125,
			},
			wantErr: "latitudes",
		},
		{
			name: "longitudes non croissantes",
			grid: func() GRIB2WindGrid {
				grid := spatialTestGrid()
				grid.Longitudes[1] = 110
				grid.Longitudes[3] = 110
				return grid
			}(),
			position: navigation.Position{
				Latitude:  5,
				Longitude: 125,
			},
			wantErr: "longitudes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := test.grid.ComponentsAt(test.position)
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %q, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func spatialTestGrid() GRIB2WindGrid {
	return GRIB2WindGrid{
		ValidAt:    time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
		Width:      2,
		Height:     2,
		Latitudes:  []float64{10, 10, 0, 0},
		Longitudes: []float64{120, 130, 120, 130},
		U:          []float64{0, 10, 20, 30},
		V:          []float64{100, 110, 120, 130},
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, attendu %.12f", name, got, want)
	}
}

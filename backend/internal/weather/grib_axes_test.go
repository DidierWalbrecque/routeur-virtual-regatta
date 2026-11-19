package weather

import (
	"math"
	"strings"
	"testing"
)

func TestRegularGridAxesExtractsRowAndColumnAxes(t *testing.T) {
	latitudes, longitudes, err := regularGridAxes(
		[]float64{10, 10, 10, 0, 0, 0},
		[]float64{120, 125, 130, 120, 125, 130},
		3,
		2,
	)
	if err != nil {
		t.Fatalf("regularGridAxes() erreur inattendue: %v", err)
	}
	if want := []float64{10, 0}; !equalFloats(latitudes, want) {
		t.Fatalf("latitudes = %v, attendu %v", latitudes, want)
	}
	if want := []float64{120, 125, 130}; !equalFloats(longitudes, want) {
		t.Fatalf("longitudes = %v, attendu %v", longitudes, want)
	}
}

func TestRegularGridAxesAcceptsFloat32Points(t *testing.T) {
	latitudes, longitudes, err := regularGridAxes(
		[]float32{10, 10, 0, 0},
		[]float32{120, 130, 120, 130},
		2,
		2,
	)
	if err != nil {
		t.Fatalf("regularGridAxes() erreur inattendue: %v", err)
	}
	if !equalFloats(latitudes, []float64{10, 0}) || !equalFloats(longitudes, []float64{120, 130}) {
		t.Fatalf("axes = %v / %v", latitudes, longitudes)
	}
}

func TestRegularGridAxesRejectsIrregularGrids(t *testing.T) {
	tests := []struct {
		name       string
		latitudes  []float64
		longitudes []float64
		width      int
		height     int
		wantErr    string
	}{
		{
			name:       "latitude variable sur une ligne",
			latitudes:  []float64{10, 11, 0, 0},
			longitudes: []float64{120, 130, 120, 130},
			width:      2, height: 2,
			wantErr: "latitude",
		},
		{
			name:       "longitude variable sur une colonne",
			latitudes:  []float64{10, 10, 0, 0},
			longitudes: []float64{120, 130, 121, 130},
			width:      2, height: 2,
			wantErr: "longitude",
		},
		{
			name:       "nombre de points incohérent",
			latitudes:  []float64{10, 10, 0},
			longitudes: []float64{120, 130, 120},
			width:      2, height: 2,
			wantErr: "attendu 4",
		},
		{
			name:       "dimensions nulles",
			latitudes:  nil,
			longitudes: nil,
			width:      0, height: 2,
			wantErr: "dimensions",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := regularGridAxes(test.latitudes, test.longitudes, test.width, test.height)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, attendu une erreur contenant %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateRejectsPerPointCoordinates(t *testing.T) {
	grid := GRIB2WindGrid{
		ValidAt:    spatialTestGrid().ValidAt,
		Width:      2,
		Height:     2,
		Latitudes:  []float64{10, 10, 0, 0},
		Longitudes: []float64{120, 130},
		U:          []float32{0, 0, 0, 0},
		V:          []float32{0, 0, 0, 0},
	}
	if err := grid.Validate(); err == nil || !strings.Contains(err.Error(), "axe des latitudes") {
		t.Fatalf("Validate() = %v, attendu une erreur sur l'axe des latitudes", err)
	}
	grid.Latitudes = []float64{10, 0}
	grid.Longitudes = []float64{120, 130, 120, 130}
	if err := grid.Validate(); err == nil || !strings.Contains(err.Error(), "axe des longitudes") {
		t.Fatalf("Validate() = %v, attendu une erreur sur l'axe des longitudes", err)
	}
}

func equalFloats(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestWindComponentToFloat32(t *testing.T) {
	compact, err := windComponentToFloat32([]float64{-12.3, 0, 25.6})
	if err != nil {
		t.Fatalf("windComponentToFloat32() erreur inattendue: %v", err)
	}
	if want := []float32{-12.3, 0, 25.6}; len(compact) != len(want) ||
		compact[0] != want[0] || compact[1] != want[1] || compact[2] != want[2] {
		t.Fatalf("compact = %v, attendu %v", compact, want)
	}

	for _, value := range []float64{math.NaN(), math.Inf(-1), 1e40} {
		if _, err := windComponentToFloat32([]float64{1, value}); err == nil ||
			!strings.Contains(err.Error(), "point 1") {
			t.Fatalf("valeur %v : erreur = %v, attendu un refus au point 1", value, err)
		}
	}
}

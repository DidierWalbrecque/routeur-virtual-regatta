package weather

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestNewGRIB2WeatherProviderSortsGrids(t *testing.T) {
	first := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	second := first.Add(3 * time.Hour)

	provider, err := NewGRIB2WeatherProvider(
		temporalTestGrid(second, 2, 3),
		temporalTestGrid(first, 0, 1),
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	if !provider.grids[0].ValidAt.Equal(first) ||
		!provider.grids[1].ValidAt.Equal(second) {
		t.Fatalf(
			"échéances non triées: %s, %s",
			provider.grids[0].ValidAt,
			provider.grids[1].ValidAt,
		)
	}
}

func TestGRIB2WeatherProviderWindAtExactTime(t *testing.T) {
	at := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	provider, err := NewGRIB2WeatherProvider(
		temporalTestGrid(at, -3, -4),
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	wind, err := provider.WindAt(
		context.Background(),
		temporalTestPosition(),
		at,
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}

	assertClose(t, "vitesse", float64(wind.Speed), 5)
	assertClose(
		t,
		"direction",
		float64(wind.Direction),
		36.86989764584402,
	)
}

func TestGRIB2WeatherProviderInterpolatesComponentsInTime(t *testing.T) {
	first := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	second := first.Add(3 * time.Hour)
	provider, err := NewGRIB2WeatherProvider(
		temporalTestGrid(first, -1, 0),
		temporalTestGrid(second, 1, 0),
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	wind, err := provider.WindAt(
		context.Background(),
		temporalTestPosition(),
		first.Add(90*time.Minute),
	)
	if err != nil {
		t.Fatalf("interpolation refusée: %v", err)
	}

	assertClose(t, "vitesse", float64(wind.Speed), 0)
	assertClose(t, "direction", float64(wind.Direction), 0)
}

func TestGRIB2WeatherProviderCombinesSpatialAndTemporalInterpolation(
	t *testing.T,
) {
	first := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	second := first.Add(3 * time.Hour)

	before := temporalTestGrid(first, 0, 10)
	before.U = []float32{0, 10, 20, 30}
	before.V = []float32{10, 20, 30, 40}

	after := temporalTestGrid(second, 40, 50)
	after.U = []float32{40, 50, 60, 70}
	after.V = []float32{50, 60, 70, 80}

	provider, err := NewGRIB2WeatherProvider(before, after)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	wind, err := provider.WindAt(
		context.Background(),
		temporalTestPosition(),
		first.Add(90*time.Minute),
	)
	if err != nil {
		t.Fatalf("interpolation refusée: %v", err)
	}

	want, err := navigation.WindFromComponents(35, 45)
	if err != nil {
		t.Fatalf("vent attendu invalide: %v", err)
	}
	assertClose(t, "vitesse", float64(wind.Speed), float64(want.Speed))
	assertClose(
		t,
		"direction",
		float64(wind.Direction),
		float64(want.Direction),
	)
}

func TestNewGRIB2WeatherProviderRejectsInvalidInputs(t *testing.T) {
	at := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		grids   []GRIB2WindGrid
		wantErr string
	}{
		{
			name:    "aucune grille",
			wantErr: "au moins une grille",
		},
		{
			name: "échéance dupliquée",
			grids: []GRIB2WindGrid{
				temporalTestGrid(at, 0, 0),
				temporalTestGrid(at, 1, 1),
			},
			wantErr: "plusieurs grilles",
		},
		{
			name: "grille invalide",
			grids: []GRIB2WindGrid{
				func() GRIB2WindGrid {
					grid := temporalTestGrid(at, 0, 0)
					grid.U = grid.U[:3]
					return grid
				}(),
			},
			wantErr: "invalide",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGRIB2WeatherProvider(test.grids...)
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %q, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestGRIB2WeatherProviderWindAtRejectsInvalidRequests(t *testing.T) {
	at := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	provider, err := NewGRIB2WeatherProvider(
		temporalTestGrid(at, 0, 0),
		temporalTestGrid(at.Add(3*time.Hour), 1, 1),
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name     string
		provider *GRIB2WeatherProvider
		ctx      context.Context
		position navigation.Position
		instant  time.Time
		wantErr  string
	}{
		{
			name:     "fournisseur nul",
			ctx:      context.Background(),
			position: temporalTestPosition(),
			instant:  at,
			wantErr:  "non initialisé",
		},
		{
			name:     "contexte nul",
			provider: provider,
			position: temporalTestPosition(),
			instant:  at,
			wantErr:  "contexte",
		},
		{
			name:     "contexte annulé",
			provider: provider,
			ctx:      cancelled,
			position: temporalTestPosition(),
			instant:  at,
			wantErr:  "annulé",
		},
		{
			name:     "position invalide",
			provider: provider,
			ctx:      context.Background(),
			position: navigation.Position{
				Latitude:  navigation.Latitude(math.NaN()),
				Longitude: 125,
			},
			instant: at,
			wantErr: "position",
		},
		{
			name:     "instant nul",
			provider: provider,
			ctx:      context.Background(),
			position: temporalTestPosition(),
			wantErr:  "instant",
		},
		{
			name:     "avant la période",
			provider: provider,
			ctx:      context.Background(),
			position: temporalTestPosition(),
			instant:  at.Add(-time.Second),
			wantErr:  "hors de la période",
		},
		{
			name:     "après la période",
			provider: provider,
			ctx:      context.Background(),
			position: temporalTestPosition(),
			instant:  at.Add(3*time.Hour + time.Second),
			wantErr:  "hors de la période",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.provider.WindAt(
				test.ctx,
				test.position,
				test.instant,
			)
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %q, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadGRIB2WeatherProviderRequiresFiles(t *testing.T) {
	_, err := LoadGRIB2WeatherProvider()
	if err == nil {
		t.Fatal("erreur attendue, aucune reçue")
	}
	if !strings.Contains(err.Error(), "au moins un fichier") {
		t.Fatalf("erreur inattendue: %v", err)
	}
}

func temporalTestGrid(
	at time.Time,
	u,
	v float32,
) GRIB2WindGrid {
	return GRIB2WindGrid{
		ValidAt:    at,
		Width:      2,
		Height:     2,
		Latitudes:  []float64{10, 0},
		Longitudes: []float64{120, 130},
		U:          []float32{u, u, u, u},
		V:          []float32{v, v, v, v},
	}
}

func temporalTestPosition() navigation.Position {
	return navigation.Position{
		Latitude:  5,
		Longitude: 125,
	}
}

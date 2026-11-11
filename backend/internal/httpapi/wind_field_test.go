package httpapi

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestWindFieldFilesKeepsTheTwoStepsAroundTheInstant(t *testing.T) {
	files := []string{"grib/gfs.2026111112.f006.grb2", "grib/gfs.2026111112.f000.grb2", "grib/gfs.2026111112.f003.grb2"}
	run := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

	selected, used := windFieldFiles(files, run.Add(4*time.Hour))
	if len(selected) != 2 || selected[0] != files[2] || selected[1] != files[0] || !used.Equal(run.Add(4*time.Hour)) {
		t.Fatalf("entre deux échéances : %v %v", selected, used)
	}
	selected, used = windFieldFiles(files, run.Add(3*time.Hour))
	if len(selected) != 1 || selected[0] != files[2] || !used.Equal(run.Add(3*time.Hour)) {
		t.Fatalf("sur une échéance : %v %v", selected, used)
	}
	// Hors de la période couverte, l'instant est ramené à l'échéance la plus proche.
	selected, used = windFieldFiles(files, run.Add(-time.Hour))
	if len(selected) != 1 || selected[0] != files[1] || !used.Equal(run) {
		t.Fatalf("avant le run : %v %v", selected, used)
	}
	selected, used = windFieldFiles(files, run.Add(30*time.Hour))
	if len(selected) != 1 || selected[0] != files[0] || !used.Equal(run.Add(6*time.Hour)) {
		t.Fatalf("après le run : %v %v", selected, used)
	}
	if selected, _ = windFieldFiles([]string{"autre.grb2"}, run); len(selected) != 0 {
		t.Fatalf("fichier sans échéance : %v", selected)
	}
}

// latitudeWind souffle à autant de mètres par seconde que la latitude, et
// manque au-delà de 60° nord.
type latitudeWind struct{}

func (latitudeWind) WindAt(_ context.Context, position navigation.Position, _ time.Time) (navigation.Wind, error) {
	if position.Latitude > 60 {
		return navigation.Wind{}, fmt.Errorf("hors grille")
	}
	return navigation.Wind{Speed: navigation.MetersPerSecond(math.Abs(float64(position.Latitude)))}, nil
}

func TestSampleWindFieldSpacesRowsLikeTheMap(t *testing.T) {
	bounds := windFieldBounds{South: 0, North: 70, West: -10, East: 10}
	speeds, err := sampleWindField(context.Background(), latitudeWind{}, time.Now(), bounds, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(speeds) != 9 {
		t.Fatalf("%d valeurs au lieu de 9", len(speeds))
	}
	// Première ligne au nord (70°, hors grille), dernière au sud (0°).
	if speeds[0] != -1 || speeds[8] != 0 {
		t.Fatalf("lignes du nord au sud attendues : %v", speeds)
	}
	// La ligne du milieu est à mi-hauteur en Mercator, pas à 35°.
	middle := latitudeFromMercatorY(mercatorY(70) / 2)
	want := math.Round(middle*3600/1852*10) / 10
	if speeds[4] != want || middle < 40 {
		t.Fatalf("ligne du milieu : %v nœuds, attendu %v (latitude %.1f)", speeds[4], want, middle)
	}
}

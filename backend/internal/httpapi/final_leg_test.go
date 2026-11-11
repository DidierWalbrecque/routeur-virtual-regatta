package httpapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

type steadyWind struct{ wind navigation.Wind }

func (provider steadyWind) WindAt(context.Context, navigation.Position, time.Time) (navigation.Wind, error) {
	return provider.wind, nil
}

func TestFinalLegPointDescribesTheLastLeg(t *testing.T) {
	boatPolar, err := polar.LoadFile(filepath.Join("..", "..", "..", "data", "polars", "imoca_60_foils.json"))
	if err != nil {
		t.Fatal(err)
	}
	owned, err := equipment.ParseSet("")
	if err != nil {
		t.Fatal(err)
	}
	from := navigation.Position{Latitude: 16.43, Longitude: -61.80}
	destination := navigation.Position{Latitude: 16.37, Longitude: -61.78}
	instant := time.Date(2026, 11, 12, 19, 33, 0, 0, time.UTC)
	// Vent de 10 nœuds venant de l'est : le dernier bord, plein sud, est au travers.
	provider := steadyWind{navigation.Wind{Speed: 10 / 1.943844, Direction: 90}}

	point := finalLegPoint(context.Background(), provider, boatPolar, owned, from, destination, instant, 10*time.Minute, 0)

	if !point.Terminal || point.Position.Latitude != float64(destination.Latitude) {
		t.Fatalf("le point d'arrivée doit rester la destination: %+v", point)
	}
	if !point.Instant.Equal(instant.Add(10 * time.Minute)) {
		t.Fatalf("instant d'arrivée inchangé attendu, obtenu %v", point.Instant)
	}
	if point.Heading < 160 || point.Heading > 170 {
		t.Fatalf("cap direct vers la destination attendu (~163°), obtenu %.1f", point.Heading)
	}
	if point.WindAngle == nil || *point.WindAngle < 65 || *point.WindAngle > 80 {
		t.Fatalf("allure du dernier bord attendue vers 73°, obtenu %v", point.WindAngle)
	}
	if point.Sail == "" || point.SpeedKnots == nil || *point.SpeedKnots <= 0 || point.WindKnots == nil {
		t.Fatalf("voile, vitesse et vent du dernier bord attendus: %+v", point)
	}

	// Sans météo, le point d'arrivée reste nu au lieu de faire échouer la route.
	bare := finalLegPoint(context.Background(), nil, boatPolar, owned, from, destination, instant, 10*time.Minute, 0)
	if !bare.Terminal || bare.WindAngle != nil || bare.Sail != "" {
		t.Fatalf("point nu attendu sans météo: %+v", bare)
	}
}

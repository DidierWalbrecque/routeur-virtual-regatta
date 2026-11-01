package httpapi

import (
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestRoutePointFromNodeCarriesStepDetails(t *testing.T) {
	boatPolar := &polar.Polar{Foil: polar.FoilProfile{
		MaxSpeedRatio: 1.04, TWAMin: 80, TWAMax: 160, TWAMerge: 10,
		TWSMin: 11, TWSMax: 30, TWSMerge: 4,
	}}
	node := routing.IsochroneNode{
		Candidate: routing.HeadingCandidate{
			Heading: 271,
			Step: simulation.StepResult{
				Wind:         navigation.Wind{Direction: 44},
				WindKnots:    20,
				WindAngle:    120,
				Tack:         navigation.TackStarboard,
				BestSail:     polar.SailPerformance{SailName: "Light Gennaker", Speed: 26.5},
				NextInstant:  time.Date(2026, 11, 1, 16, 46, 0, 0, time.UTC),
				NextPosition: navigation.Position{Latitude: 49, Longitude: -4},
			},
		},
		Maneuver:     &simulation.ManeuverState{Kind: simulation.ManeuverGybe},
		SpeedPenalty: &simulation.SpeedPenaltyApplication{AppliedPenalty: &polar.ManeuverPenalty{SpeedRatio: 0.9}},
	}

	owned, err := equipment.ParseSet("foils")
	if err != nil {
		t.Fatal(err)
	}
	point := routePointFromNode(node, boatPolar, owned)
	if point.Tack != "starboard" || point.Maneuver != "gybe" || point.Sail != "Light Gennaker" {
		t.Fatalf("point inattendu: %+v", point)
	}
	if *point.WindAngle != 120 || *point.WindDirection != 44 || *point.WindKnots != 20 || *point.SpeedKnots != 26.5 {
		t.Fatalf("vent ou vitesse inattendus: %+v", point)
	}
	if point.FoilPercent == nil || *point.FoilPercent < 99 || *point.FoilPercent > 100 {
		t.Fatalf("foils attendus à 100 %% en plein dans la zone, obtenu %v", point.FoilPercent)
	}
	if point.SpeedCoefficient == nil || *point.SpeedCoefficient != 0.9 {
		t.Fatalf("coefficient attendu 0.9, obtenu %v", point.SpeedCoefficient)
	}

	noEquipment, err := equipment.ParseSet("")
	if err != nil {
		t.Fatal(err)
	}
	without := routePointFromNode(node, boatPolar, noEquipment)
	if without.FoilPercent != nil {
		t.Fatalf("sans foils, aucun pourcentage attendu, obtenu %v", *without.FoilPercent)
	}
}

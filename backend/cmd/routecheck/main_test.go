package main

import (
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestParseOptionsAcceptsVirtualRegattaPositions(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f000.grb2",
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-position", `13°N 07'16" - 121°E 35'12"`,
		"-destination", `14°N 00'00" - 120°E 00'00"`,
		"-at", "2026-09-05T07:30:00Z",
		"-minutes", "240",
		"-step-minutes", "10",
		"-heading-spacing", "15",
		"-cell-size", "2.5",
		"-max-candidates", "100",
		"-arrival-radius", "0.5",
		"-geojson", "out/route.geojson",
		"-owned", "code0,foils",
	}, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if len(options.files) != 2 {
		t.Fatalf("fichiers = %v", options.files)
	}
	if options.polarPath != "boat.json" {
		t.Errorf("polaire = %q, attendue boat.json", options.polarPath)
	}
	assertClose(
		t,
		"latitude initiale",
		float64(options.position.Latitude),
		13.12111111111111,
	)
	assertClose(
		t,
		"longitude initiale",
		float64(options.position.Longitude),
		121.58666666666667,
	)
	if options.destination != (navigation.Position{
		Latitude:  14,
		Longitude: 120,
	}) {
		t.Errorf("destination = %+v", options.destination)
	}
	if options.totalDuration != 4*time.Hour {
		t.Errorf("durée totale = %s, attendue 4h", options.totalDuration)
	}
	if options.stepDuration != 10*time.Minute {
		t.Errorf("durée du pas = %s, attendue 10m", options.stepDuration)
	}
	if options.layerCount != 24 {
		t.Errorf("fronts = %d, attendus 24", options.layerCount)
	}
	if options.headingSpacing != 15 {
		t.Errorf("écart de caps = %.1f°, attendu 15°", options.headingSpacing)
	}
	if options.cellSize != 2.5 {
		t.Errorf("taille de cellule = %.3f NM, attendue 2.5", options.cellSize)
	}
	if options.maxCandidates != 100 {
		t.Errorf(
			"limite = %d, attendue 100",
			options.maxCandidates,
		)
	}
	if options.arrivalRadius != 0.5 {
		t.Errorf(
			"rayon d'arrivée = %.3f NM, attendu 0.5",
			options.arrivalRadius,
		)
	}
	if options.geoJSONPath != "out/route.geojson" {
		t.Errorf(
			"GeoJSON = %q, attendu out/route.geojson",
			options.geoJSONPath,
		)
	}
	if !options.owned.Has(equipment.Code0) ||
		!options.owned.Has(equipment.Foils) {
		t.Errorf("équipements = %v", options.owned)
	}
}

func TestParseOptionsAcceptsDecimalPositionsAndDefaults(t *testing.T) {
	options, err := parseOptions([]string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-latitude", "13.121111",
		"-longitude", "121.586667",
		"-destination-latitude", "14",
		"-destination-longitude", "120",
		"-at", "2026-09-05T09:00:00Z",
		"-minutes", "60",
	}, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if options.layerCount != 6 {
		t.Errorf("fronts = %d, attendus 6", options.layerCount)
	}
	if options.stepDuration != 10*time.Minute {
		t.Errorf("durée du pas = %s, attendue 10m", options.stepDuration)
	}
	if options.headingSpacing != 10 {
		t.Errorf("écart de caps = %.1f°, attendu 10°", options.headingSpacing)
	}
	if options.cellSize != 1 {
		t.Errorf("taille de cellule = %.3f NM, attendue 1", options.cellSize)
	}
	if options.maxCandidates != 500 {
		t.Errorf(
			"limite = %d, attendue 500",
			options.maxCandidates,
		)
	}
	if options.arrivalRadius != 1 {
		t.Errorf(
			"rayon d'arrivée = %.3f NM, attendu 1",
			options.arrivalRadius,
		)
	}
	if options.geoJSONPath != "" {
		t.Errorf(
			"GeoJSON par défaut = %q, attendu vide",
			options.geoJSONPath,
		)
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	base := []string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-position", `13°N 07'16" - 121°E 35'12"`,
		"-destination", `14°N 00'00" - 120°E 00'00"`,
		"-at", "2026-09-05T09:00:00Z",
		"-minutes", "60",
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "fichier absent",
			args:    base[2:],
			wantErr: "-file",
		},
		{
			name: "polaire absente",
			args: []string{
				"-file", "f003.grb2",
				"-position", `13°N 07'16" - 121°E 35'12"`,
				"-destination", `14°N 00'00" - 120°E 00'00"`,
				"-at", "2026-09-05T09:00:00Z",
				"-minutes", "60",
			},
			wantErr: "-polar",
		},
		{
			name: "position initiale absente",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-destination", `14°N 00'00" - 120°E 00'00"`,
				"-at", "2026-09-05T09:00:00Z",
				"-minutes", "60",
			},
			wantErr: "position initiale",
		},
		{
			name: "destination absente",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-position", `13°N 07'16" - 121°E 35'12"`,
				"-at", "2026-09-05T09:00:00Z",
				"-minutes", "60",
			},
			wantErr: "destination",
		},
		{
			name: "instant absent",
			args: []string{
				"-file", "f003.grb2",
				"-polar", "boat.json",
				"-position", `13°N 07'16" - 121°E 35'12"`,
				"-destination", `14°N 00'00" - 120°E 00'00"`,
				"-minutes", "60",
			},
			wantErr: "-at",
		},
		{
			name: "durée non multiple du pas",
			args: append(
				append([]string{}, base...),
				"-step-minutes", "7",
			),
			wantErr: "multiple exact",
		},
		{
			name: "écart de caps invalide",
			args: append(
				append([]string{}, base...),
				"-heading-spacing", "7",
			),
			wantErr: "-heading-spacing",
		},
		{
			name: "cellule nulle",
			args: append(
				append([]string{}, base...),
				"-cell-size", "0",
			),
			wantErr: "-cell-size",
		},
		{
			name: "limite de candidats trop petite",
			args: append(
				append([]string{}, base...),
				"-max-candidates", "3",
			),
			wantErr: "-max-candidates",
		},
		{
			name: "rayon d'arrivée négatif",
			args: append(
				append([]string{}, base...),
				"-arrival-radius", "-1",
			),
			wantErr: "-arrival-radius",
		},
		{
			name: "équipement inconnu",
			args: append(
				append([]string{}, base...),
				"-owned", "radio",
			),
			wantErr: "équipements possédés",
		},
		{
			name: "formats de destination mélangés",
			args: append(
				append([]string{}, base...),
				"-destination-latitude", "14",
				"-destination-longitude", "120",
			),
			wantErr: "ne peut pas être combiné",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions(test.args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestWriteResult(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{Latitude: 13, Longitude: 121}
	destination := navigation.Position{Latitude: 14, Longitude: 120}
	nextPosition := navigation.Position{
		Latitude:  13.01,
		Longitude: 120.99,
	}
	node := routing.IsochroneNode{
		ParentIndex: -1,
		Candidate: routing.HeadingCandidate{
			Heading: 321,
			Step: simulation.StepResult{
				Position:     position,
				Instant:      instant,
				Heading:      321,
				Duration:     10 * time.Minute,
				Wind:         navigation.Wind{Speed: 4, Direction: 343},
				WindKnots:    7.77538,
				WindAngle:    22,
				Tack:         navigation.TackStarboard,
				BestSail:     polar.SailPerformance{SailName: "Jib", Speed: 8},
				NextPosition: nextPosition,
				NextInstant:  instant.Add(10 * time.Minute),
			},
		},
		InitialDistance:   84,
		RemainingDistance: 83,
		Progress:          1,
		Maneuver: &simulation.ManeuverState{
			Kind:        simulation.ManeuverTack,
			SailChanged: true,
		},
		State: simulation.BoatState{
			Stamina: simulation.StaminaState{Level: 72.5},
		},
	}
	search := routing.IsochroneSearch{
		Origin:        position,
		Destination:   destination,
		StartedAt:     instant,
		StepDuration:  10 * time.Minute,
		MaxCandidates: 500,
		Frontiers: []routing.IsochroneFrontier{
			{
				Index:    1,
				Instant:  instant.Add(10 * time.Minute),
				CellSize: 1,
				Nodes:    []routing.IsochroneNode{node},
			},
		},
	}
	route := routing.IsochroneRoute{
		Origin:      position,
		Destination: destination,
		StartedAt:   instant,
		FinishedAt:  instant.Add(10 * time.Minute),
		Steps: []routing.IsochroneRouteStep{
			{
				FrontierIndex: 0,
				NodeIndex:     0,
				Node:          node,
			},
		},
	}
	options := options{
		position:       position,
		destination:    destination,
		instant:        instant,
		totalDuration:  10 * time.Minute,
		stepDuration:   10 * time.Minute,
		layerCount:     1,
		headingSpacing: 10,
		cellSize:       1,
		maxCandidates:  500,
		arrivalRadius:  1,
	}

	var output strings.Builder
	err := writeResult(
		&output,
		&polar.Polar{Label: "Test boat"},
		options,
		search,
		route,
	)
	if err != nil {
		t.Fatalf("écriture refusée: %v", err)
	}

	for _, expected := range []string{
		"Bateau : Test boat",
		`Départ VR : 13°N 00'00" - 121°E 00'00"`,
		`Destination VR : 14°N 00'00" - 120°E 00'00"`,
		"Route retenue :",
		"1. 2026-09-05T07:40:00Z | cap 321.00°",
		"voile Jib | vitesse 8.000 nd",
		"distance restante 83.000 NM",
		"manœuvre virement | changement de voile oui | stamina 72.5",
		"Position finale VR :",
		"Destination atteinte : non",
		"Positions conservées par front : 1",
		"Cellules effectives par front : 1.000",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("sortie absente %q:\n%s", expected, output.String())
		}
	}
}

func TestFormatManeuverState(t *testing.T) {
	tests := []struct {
		name           string
		maneuver       *simulation.ManeuverState
		wantManeuver   string
		wantSailChange string
	}{
		{
			name:           "inconnu",
			wantManeuver:   "indéterminée",
			wantSailChange: "indéterminé",
		},
		{
			name: "aucune action",
			maneuver: &simulation.ManeuverState{
				Kind: simulation.ManeuverNone,
			},
			wantManeuver:   "aucune",
			wantSailChange: "non",
		},
		{
			name: "empannage et voile",
			maneuver: &simulation.ManeuverState{
				Kind:        simulation.ManeuverGybe,
				SailChanged: true,
			},
			wantManeuver:   "empannage",
			wantSailChange: "oui",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatManeuver(test.maneuver); got != test.wantManeuver {
				t.Errorf("manœuvre = %q, attendue %q", got, test.wantManeuver)
			}
			if got := formatSailChange(test.maneuver); got != test.wantSailChange {
				t.Errorf(
					"changement de voile = %q, attendu %q",
					got,
					test.wantSailChange,
				)
			}
		})
	}
}

func assertClose(
	t *testing.T,
	name string,
	got,
	want float64,
) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.12f, attendu %.12f", name, got, want)
	}
}

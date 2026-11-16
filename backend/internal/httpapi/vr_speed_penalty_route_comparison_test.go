package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// Compare l'effet des règles de cumul des pénalités sur des routes complètes,
// avec les prévisions réelles de data/weather. Ce calcul est long et dépend de
// fichiers locaux : il ne tourne que sur demande.
//
//	VR_ROUTE_COMPARISON=quick go test ./internal/httpapi/ -run TestVRSpeedPenaltyRuleRouteComparison -v -timeout 60m
//
// La carte des terres n'est pas utilisée : les parcours restent au large et
// visent une destination hors de portée, pour comparer des milles restants
// plutôt qu'une heure d'arrivée arrondie au pas de calcul.

const comparisonHorizon = 48 * time.Hour

// Le moteur attend une seule pénalité. Les règles de comparaison sont donc
// traduites en une pénalité de même perte de distance tant qu'elle tient dans
// le pas ; seule la règle actuelle est exacte quand une pénalité déborde.
func equivalentPenalty(duration time.Duration, lostSeconds float64) polar.ManeuverPenalty {
	return polar.ManeuverPenalty{
		SpeedRatio: polar.SpeedRatio(1 - lostSeconds/duration.Seconds()),
		Duration:   duration,
	}
}

func combineSequentialPenalties(first, second polar.ManeuverPenalty) (polar.ManeuverPenalty, error) {
	lost := (1-float64(first.SpeedRatio))*first.Duration.Seconds() +
		(1-float64(second.SpeedRatio))*second.Duration.Seconds()
	return equivalentPenalty(first.Duration+second.Duration, lost), nil
}

func combineOverlappedPenalties(first, second polar.ManeuverPenalty) (polar.ManeuverPenalty, error) {
	shorter, longer := first, second
	if shorter.Duration > longer.Duration {
		shorter, longer = longer, shorter
	}
	lost := (1-float64(first.SpeedRatio)*float64(second.SpeedRatio))*shorter.Duration.Seconds() +
		(1-float64(longer.SpeedRatio))*(longer.Duration-shorter.Duration).Seconds()
	return equivalentPenalty(longer.Duration, lost), nil
}

func comparisonWeather(t *testing.T) (weather.WeatherProvider, time.Time) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "data", "weather", "gfs.*.f*.grb2"))
	if err != nil {
		t.Fatal(err)
	}
	runs := map[string][]string{}
	for _, path := range paths {
		parts := strings.Split(filepath.Base(path), ".")
		hour, err := strconv.Atoi(strings.TrimPrefix(parts[2], "f"))
		if err != nil {
			t.Fatalf("échéance illisible dans %q: %v", path, err)
		}
		if hour <= 96 {
			runs[parts[1]] = append(runs[parts[1]], path)
		}
	}
	selected := ""
	for run, files := range runs {
		// 33 fichiers : f000 à f096 toutes les 3 heures.
		if len(files) == 33 && run > selected {
			selected = run
		}
	}
	if selected == "" {
		t.Skip("aucun run GFS complet sur 96 heures dans data/weather")
	}
	sort.Strings(runs[selected])
	started, err := time.Parse("2006010215", selected)
	if err != nil {
		t.Fatal(err)
	}
	provider, _, err := loadCachedWeatherProvider(runs[selected]...)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("run GFS %s, départ %s", selected, started.Add(6*time.Hour).Format(time.RFC3339))
	return provider, started.Add(6 * time.Hour)
}

type routeTally struct {
	remaining        float64
	maneuvers, sails int
	simultaneous     int
	overlaps         int
	finalStamina     float64
	positions        []navigation.Position
	reached          bool
	elapsed          time.Duration
}

func tallyRoute(t *testing.T, search routing.IsochroneSearch, elapsed time.Duration) routeTally {
	t.Helper()
	route, err := search.ReconstructBestRoute()
	if err != nil {
		t.Fatalf("reconstruction impossible: %v", err)
	}
	last := route.Steps[len(route.Steps)-1].Node
	tally := routeTally{
		remaining:    float64(last.RemainingDistance),
		finalStamina: float64(last.State.Stamina.Level),
		positions:    route.Positions(),
		reached:      search.Reached,
		elapsed:      elapsed,
	}
	for index, step := range route.Steps {
		maneuver := step.Node.Maneuver
		if maneuver == nil {
			continue
		}
		turned := maneuver.Kind == simulation.ManeuverTack || maneuver.Kind == simulation.ManeuverGybe
		if turned {
			tally.maneuvers++
		}
		if maneuver.SailChanged {
			tally.sails++
		}
		if (turned || maneuver.SailChanged) && index > 0 && route.Steps[index-1].Node.State.ActiveSpeedPenalty != nil {
			tally.overlaps++
		}
		if turned && maneuver.SailChanged {
			tally.simultaneous++
		}
	}
	return tally
}

func largestSeparation(t *testing.T, a, b []navigation.Position) float64 {
	t.Helper()
	largest := 0.0
	for index := 0; index < len(a) && index < len(b); index++ {
		distance, err := navigation.DistanceBetween(a[index], b[index])
		if err != nil {
			t.Fatal(err)
		}
		if float64(distance) > largest {
			largest = float64(distance)
		}
	}
	return largest
}

type comparisonEquipment struct {
	name  string
	owned equipment.EquipmentSet
}

var comparisonCourses = []struct {
	name         string
	origin, dest navigation.Position
}{
	{"Atlantique Nord vers l'ouest", navigation.Position{Latitude: 45, Longitude: -15}, navigation.Position{Latitude: 40, Longitude: -65}},
	{"Alizes vers l'ouest", navigation.Position{Latitude: 25, Longitude: -25}, navigation.Position{Latitude: 12, Longitude: -58}},
	{"Ocean Austral vers l'est", navigation.Position{Latitude: -45, Longitude: 30}, navigation.Position{Latitude: -47, Longitude: 110}},
	{"Pacifique Nord vers l'ouest", navigation.Position{Latitude: 30, Longitude: -130}, navigation.Position{Latitude: 30, Longitude: -175}},
}

// Mesure si les routes s'améliorent puis se stabilisent quand le front garde
// plus de positions, avec la règle de cumul actuelle.
//
//	VR_MAX_CANDIDATES_COMPARISON=quick go test ./internal/httpapi/ -run TestVRMaxCandidatesRouteConvergence -v -timeout 240m
func TestVRMaxCandidatesRouteConvergence(t *testing.T) {
	mode := os.Getenv("VR_MAX_CANDIDATES_COMPARISON")
	if mode == "" {
		t.Skip("définir VR_MAX_CANDIDATES_COMPARISON=quick ou precise pour lancer cette comparaison")
	}
	profile, err := calculationProfileFor(mode)
	if err != nil {
		t.Fatal(err)
	}
	boatPolar := loadCombinationPolar(t)
	provider, instant := comparisonWeather(t)
	evaluator, combine := staminaManeuverRuntime(boatPolar)
	recovery := staminaRecoveryRules()

	withoutPro, err := equipment.NewSet(equipment.Light, equipment.Heavy, equipment.Code0, equipment.Foils, equipment.HullPolish)
	if err != nil {
		t.Fatal(err)
	}
	withPro, err := equipment.NewSet(equipment.All()...)
	if err != nil {
		t.Fatal(err)
	}
	equipments := []comparisonEquipment{
		{"winchs standard", withoutPro},
		{"winchs pro", withPro},
	}

	layers := int(comparisonHorizon / profile.IsochroneStep)
	for _, course := range comparisonCourses {
		for _, gear := range equipments {
			t.Run(course.name+", "+gear.name, func(t *testing.T) {
				var reference routeTally
				for index, maxCandidates := range []int{300, 600, 1200, 2400} {
					started := time.Now()
					search, err := routing.BuildIsochroneSearch(context.Background(), provider, boatPolar, routing.IsochroneSearchInput{
						Position: course.origin, Destination: course.dest, Instant: instant,
						StepDuration: profile.IsochroneStep, LayerCount: layers, Owned: gear.owned,
						HeadingSpacing: profile.HeadingSpacing, RefineHeadings: true,
						CellSize: profile.CellSize, MaxCandidates: maxCandidates,
						ArrivalRadius:         1,
						InitialStamina:        simulation.StaminaState{Level: 100},
						TransitionEvaluator:   evaluator,
						CombineSpeedPenalties: combine,
						StaminaRecoveryRules:  &recovery,
					})
					if err != nil {
						t.Fatalf("%d positions: calcul impossible: %v", maxCandidates, err)
					}
					tally := tallyRoute(t, search, time.Since(started))
					if tally.reached {
						t.Errorf("%d positions: destination atteinte, la comparaison en milles restants ne vaut plus", maxCandidates)
					}
					if index == 0 {
						reference = tally
					}
					final := len(search.Frontiers[len(search.Frontiers)-1].Nodes)
					t.Logf("%4d positions (dernier front %4d) restant %8.2f NM (gain %+6.2f) | virements/empannages %2d, voiles %2d, simultanes %2d, chevauchements %2d | stamina %5.1f | ecart max %6.1f NM | %v",
						maxCandidates, final, tally.remaining, reference.remaining-tally.remaining,
						tally.maneuvers, tally.sails, tally.simultaneous, tally.overlaps, tally.finalStamina,
						largestSeparation(t, reference.positions, tally.positions), tally.elapsed.Round(time.Second))
				}
			})
		}
	}
}

func TestVRSpeedPenaltyRuleRouteComparison(t *testing.T) {
	mode := os.Getenv("VR_ROUTE_COMPARISON")
	if mode == "" {
		t.Skip("définir VR_ROUTE_COMPARISON=quick ou precise pour lancer cette comparaison")
	}
	profile, err := calculationProfileFor(mode)
	if err != nil {
		t.Fatal(err)
	}
	boatPolar := loadCombinationPolar(t)
	provider, instant := comparisonWeather(t)
	evaluator, _ := staminaManeuverRuntime(boatPolar)
	recovery := staminaRecoveryRules()

	withoutPro, err := equipment.NewSet(equipment.Light, equipment.Heavy, equipment.Code0, equipment.Foils, equipment.HullPolish)
	if err != nil {
		t.Fatal(err)
	}
	withPro, err := equipment.NewSet(equipment.All()...)
	if err != nil {
		t.Fatal(err)
	}

	rules := []struct {
		name    string
		combine simulation.ManeuverSpeedPenaltyCombiner
	}{
		{"actuelle", combineVRSpeedPenalties},
		{"a la suite", combineSequentialPenalties},
		{"simultanee", combineOverlappedPenalties},
	}
	equipments := []comparisonEquipment{
		{"winchs standard", withoutPro},
		{"winchs pro", withPro},
	}

	layers := int(comparisonHorizon / profile.IsochroneStep)
	for _, course := range comparisonCourses {
		for _, gear := range equipments {
			t.Run(course.name+", "+gear.name, func(t *testing.T) {
				var reference routeTally
				for index, rule := range rules {
					started := time.Now()
					search, err := routing.BuildIsochroneSearch(context.Background(), provider, boatPolar, routing.IsochroneSearchInput{
						Position: course.origin, Destination: course.dest, Instant: instant,
						StepDuration: profile.IsochroneStep, LayerCount: layers, Owned: gear.owned,
						HeadingSpacing: profile.HeadingSpacing, RefineHeadings: true,
						CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
						ArrivalRadius:         1,
						InitialStamina:        simulation.StaminaState{Level: 100},
						TransitionEvaluator:   evaluator,
						CombineSpeedPenalties: rule.combine,
						StaminaRecoveryRules:  &recovery,
					})
					if err != nil {
						t.Fatalf("%s: calcul impossible: %v", rule.name, err)
					}
					tally := tallyRoute(t, search, time.Since(started))
					if tally.reached {
						t.Errorf("%s: destination atteinte, la comparaison en milles restants ne vaut plus", rule.name)
					}
					if index == 0 {
						reference = tally
					}
					t.Logf("%-10s restant %8.2f NM (gain %+6.2f) | virements/empannages %2d, voiles %2d, simultanes %2d, chevauchements %2d | stamina %5.1f | ecart max %6.1f NM | %v",
						rule.name, tally.remaining, reference.remaining-tally.remaining,
						tally.maneuvers, tally.sails, tally.simultaneous, tally.overlaps, tally.finalStamina,
						largestSeparation(t, reference.positions, tally.positions), tally.elapsed.Round(time.Second))
				}
			})
		}
	}
}

package routing

import (
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func rankedNode(targetProgress float64, maneuverStamina float64, stamina float64) IsochroneNode {
	return IsochroneNode{
		TargetProgress:  DistanceProgress(targetProgress),
		Progress:        DistanceProgress(targetProgress),
		ManeuverStamina: simulation.StaminaLevel(maneuverStamina),
		State: simulation.BoatState{
			Stamina: simulation.StaminaState{Level: simulation.StaminaLevel(stamina)},
		},
	}
}

func finalSearch(nodes ...IsochroneNode) IsochroneSearch {
	return IsochroneSearch{Frontiers: []IsochroneFrontier{{Index: 1, Nodes: nodes}}}
}

// Un aller-retour inutile (deux empannages, environ 24 points de stamina) qui
// ne fait gagner que quelques mètres ne doit pas être la route retenue, même
// quand la stamina s'est rechargée à 100 % à l'arrivée.
func TestBestRejectsUselessManeuvers(t *testing.T) {
	search := finalSearch(rankedNode(130.02, 24, 100), rankedNode(130.00, 0, 100))
	best, ok := search.Best()
	if !ok || best.ManeuverStamina != 0 {
		t.Fatalf("route retenue : %.0f points dépensés en manœuvres, attendu la route directe", float64(best.ManeuverStamina))
	}
	if index, _ := search.bestFinalIndex(); index != 1 {
		t.Fatalf("la reconstruction partirait du nœud %d, attendu 1", index)
	}
}

// Une manœuvre qui fait gagner plus que son coût reste retenue.
func TestBestKeepsWorthwhileManeuvers(t *testing.T) {
	search := finalSearch(rankedNode(130.0, 0, 100), rankedNode(130.5, 12, 88))
	best, _ := search.Best()
	if best.ManeuverStamina != 12 {
		t.Fatal("un virement qui fait gagner 0,5 NM pour 12 points de stamina doit être gardé")
	}
}

// Pendant le calcul, le classement reste la progression brute : une route qui
// a déjà manœuvré n'est pas pénalisée face à une voisine qui manœuvrera plus
// tard, sinon de bonnes routes seraient éliminées trop tôt.
func TestNodeBetterIgnoresManeuverCostDuringSearch(t *testing.T) {
	maneuvered := rankedNode(10.02, 24, 76)
	notYet := rankedNode(10.00, 0, 100)
	if !nodeBetter(maneuvered, notYet) {
		t.Fatal("pendant le calcul, la progression brute doit primer")
	}
}

// Une porte franchie reste le premier critère, quel que soit le coût des manœuvres.
func TestFinalNodeBetterStillPutsGatesFirst(t *testing.T) {
	ahead := rankedNode(1, 60, 40)
	ahead.CourseProgress.NextGateIndex = 2
	behind := rankedNode(50, 0, 100)
	behind.CourseProgress.NextGateIndex = 1
	if !finalNodeBetter(ahead, behind) {
		t.Fatal("le nœud qui a franchi une porte de plus doit passer devant")
	}
}

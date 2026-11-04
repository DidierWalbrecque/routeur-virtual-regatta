package routing

import (
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestPruneIsochroneNodesKeepsNondominatedBoatStates(t *testing.T) {
	bestProgress := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		50,
		10,
	)
	dominated := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		40,
		9,
	)
	moreStamina := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		80,
		8,
	)
	otherSail := stateAwarePruningNode(
		90,
		2,
		navigation.TackPort,
		30,
		7,
	)

	selected, err := pruneIsochroneNodes(
		navigation.Position{},
		[]IsochroneNode{
			dominated,
			moreStamina,
			otherSail,
			bestProgress,
		},
		10,
	)
	if err != nil {
		t.Fatalf("élagage refusé: %v", err)
	}
	if len(selected) != 3 {
		t.Fatalf(
			"nœuds conservés = %d, attendus 3",
			len(selected),
		)
	}
	if selected[0].Progress != 10 {
		t.Errorf(
			"première progression = %.1f, attendue 10",
			selected[0].Progress,
		)
	}
	if selected[1].State.Stamina.Level != 80 {
		t.Errorf(
			"seconde stamina = %.1f, attendue 80",
			selected[1].State.Stamina.Level,
		)
	}
	if selected[2].State.SailID != 2 {
		t.Errorf(
			"troisième voile = %d, attendue 2",
			selected[2].State.SailID,
		)
	}
}

func TestPruneIsochroneNodesRejectsInvalidStamina(t *testing.T) {
	node := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		-1,
		10,
	)
	_, err := pruneIsochroneNodes(
		navigation.Position{},
		[]IsochroneNode{node},
		10,
	)
	if err == nil || !strings.Contains(err.Error(), "stamina") {
		t.Fatalf(
			"erreur = %v, doit signaler la stamina",
			err,
		)
	}
}

func TestPruneIsochroneNodesToLimitKeepsHardCandidateLimit(t *testing.T) {
	nodes := make([]IsochroneNode, 6)
	for index := range nodes {
		nodes[index] = stateAwarePruningNode(
			navigation.Heading(index*60),
			1,
			navigation.TackPort,
			100,
			DistanceProgress(10-index),
		)
	}

	selected, cellSize, err := pruneIsochroneNodesToLimit(
		navigation.Position{},
		nodes,
		1,
		4,
	)
	if err != nil {
		t.Fatalf("élagage limité refusé: %v", err)
	}
	if len(selected) != 4 {
		t.Fatalf(
			"nœuds conservés = %d, limite attendue 4",
			len(selected),
		)
	}
	if cellSize != 1 {
		t.Errorf(
			"cellule finale = %.1f NM, attendue 1 NM (cap par secteurs, plus de doublement)",
			cellSize,
		)
	}
	for index, node := range selected {
		wantProgress := DistanceProgress(10 - index)
		if node.Progress != wantProgress {
			t.Errorf(
				"progression %d = %.1f, attendue %.1f",
				index,
				node.Progress,
				wantProgress,
			)
		}
	}
}

func stateAwarePruningNode(
	heading navigation.Heading,
	sailID polar.SailID,
	tack navigation.Tack,
	stamina simulation.StaminaLevel,
	progress DistanceProgress,
) IsochroneNode {
	position := navigation.Position{
		Latitude:  0.01,
		Longitude: 0.01,
	}
	return IsochroneNode{
		Candidate: HeadingCandidate{
			Heading: heading,
			Step: simulation.StepResult{
				Heading:      heading,
				NextPosition: position,
			},
		},
		Progress:       progress,
		TargetProgress: progress,
		State: simulation.BoatState{
			Heading: heading,
			SailID:  sailID,
			Tack:    tack,
			Stamina: simulation.StaminaState{Level: stamina},
		},
	}
}

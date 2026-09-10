package routing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// IsochroneSearchInput describes a multi-layer isochrone exploration.
type IsochroneSearchInput struct {
	Position       navigation.Position
	Destination    navigation.Position
	Instant        time.Time
	StepDuration   time.Duration
	LayerCount     int
	Owned          equipment.EquipmentSet
	HeadingSpacing float64
	CellSize       navigation.NauticalMiles
}

// IsochroneNode represents one retained position and its parent in the
// preceding frontier. ParentIndex is -1 for the first frontier.
type IsochroneNode struct {
	ParentIndex      int
	Candidate        HeadingCandidate
	InitialDistance  navigation.NauticalMiles
	RemainingDistance navigation.NauticalMiles
	Progress         DistanceProgress
}

// IsochroneFrontier contains the retained positions reached at one instant.
type IsochroneFrontier struct {
	Index   int
	Instant time.Time
	Nodes   []IsochroneNode
}

// IsochroneSearch contains every retained frontier of an exploration.
type IsochroneSearch struct {
	Origin       navigation.Position
	Destination  navigation.Position
	StartedAt    time.Time
	StepDuration time.Duration
	Frontiers    []IsochroneFrontier
}

// Best returns the best retained node from the final frontier.
func (search IsochroneSearch) Best() (IsochroneNode, bool) {
	if len(search.Frontiers) == 0 {
		return IsochroneNode{}, false
	}
	final := search.Frontiers[len(search.Frontiers)-1]
	if len(final.Nodes) == 0 {
		return IsochroneNode{}, false
	}
	return final.Nodes[0], true
}

// BuildIsochroneSearch expands every retained node at each step and applies
// spatial pruning before building the following frontier.
func BuildIsochroneSearch(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input IsochroneSearchInput,
) (IsochroneSearch, error) {
	if weatherProvider == nil {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: fournisseur météo requis",
		)
	}
	if boatPolar == nil {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: polaire requise",
		)
	}
	if err := input.Position.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: position initiale invalide: %w",
			err,
		)
	}
	if err := input.Destination.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: destination invalide: %w",
			err,
		)
	}
	if input.Instant.IsZero() {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: instant initial requis",
		)
	}
	if input.StepDuration <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: durée de pas strictement positive requise",
		)
	}
	if input.LayerCount <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: nombre de couches strictement positif requis",
		)
	}
	if err := input.Owned.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: équipements invalides: %w",
			err,
		)
	}

	headings, err := GenerateHeadingGrid(input.HeadingSpacing)
	if err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: grille de caps invalide: %w",
			err,
		)
	}

	cellSize := float64(input.CellSize)
	if math.IsNaN(cellSize) ||
		math.IsInf(cellSize, 0) ||
		input.CellSize <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: taille de cellule finie strictement positive requise",
		)
	}

	initialDistance, err := navigation.DistanceBetween(
		input.Position,
		input.Destination,
	)
	if err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: distance initiale impossible: %w",
			err,
		)
	}

	search := IsochroneSearch{
		Origin:       input.Position,
		Destination:  input.Destination,
		StartedAt:    input.Instant,
		StepDuration: input.StepDuration,
		Frontiers:    make([]IsochroneFrontier, 0, input.LayerCount),
	}
	parentPositions := []navigation.Position{input.Position}
	currentInstant := input.Instant

	for layerIndex := 0; layerIndex < input.LayerCount; layerIndex++ {
		expanded := make(
			[]IsochroneNode,
			0,
			len(parentPositions)*len(headings),
		)

		for parentIndex, position := range parentPositions {
			candidates, err := EvaluateHeadingCandidates(
				ctx,
				weatherProvider,
				boatPolar,
				HeadingCandidatesInput{
					Position: position,
					Instant:  currentInstant,
					Duration: input.StepDuration,
					Owned:    input.Owned,
					Headings: headings,
				},
			)
			if err != nil {
				return IsochroneSearch{}, fmt.Errorf(
					"routing: construction de la couche %d impossible: développement du parent %d impossible: %w",
					layerIndex+1,
					parentIndex+1,
					err,
				)
			}

			lineageParent := parentIndex
			if layerIndex == 0 {
				lineageParent = -1
			}

			for _, candidate := range candidates {
				remainingDistance, err := navigation.DistanceBetween(
					candidate.Step.NextPosition,
					input.Destination,
				)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: distance restante impossible: %w",
						layerIndex+1,
						err,
					)
				}

				expanded = append(expanded, IsochroneNode{
					ParentIndex:       lineageParent,
					Candidate:         candidate,
					InitialDistance:   initialDistance,
					RemainingDistance: remainingDistance,
					Progress: DistanceProgress(
						float64(initialDistance - remainingDistance),
					),
				})
			}
		}

		nodes, err := pruneIsochroneNodes(
			input.Position,
			expanded,
			cellSize,
		)
		if err != nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction de la couche %d impossible: élagage spatial impossible: %w",
				layerIndex+1,
				err,
			)
		}

		nextInstant := currentInstant.Add(input.StepDuration)
		search.Frontiers = append(search.Frontiers, IsochroneFrontier{
			Index:   layerIndex + 1,
			Instant: nextInstant,
			Nodes:   nodes,
		})

		parentPositions = make([]navigation.Position, len(nodes))
		for index, node := range nodes {
			parentPositions[index] = node.Candidate.Step.NextPosition
		}
		currentInstant = nextInstant
	}

	return search, nil
}

func pruneIsochroneNodes(
	origin navigation.Position,
	nodes []IsochroneNode,
	cellSize float64,
) ([]IsochroneNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("aucune position à conserver")
	}

	selected := make([]IsochroneNode, 0, len(nodes))
	cellIndexes := make(map[spatialCell]int, len(nodes))
	for index, node := range nodes {
		cell, err := spatialCellFor(
			origin,
			node.Candidate.Step.NextPosition,
			cellSize,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cellule de la position %d impossible: %w",
				index+1,
				err,
			)
		}

		selectedIndex, exists := cellIndexes[cell]
		if !exists {
			cellIndexes[cell] = len(selected)
			selected = append(selected, node)
			continue
		}
		if node.Progress > selected[selectedIndex].Progress {
			selected[selectedIndex] = node
		}
	}

	sort.SliceStable(selected, func(left, right int) bool {
		return selected[left].Progress > selected[right].Progress
	})
	return selected, nil
}

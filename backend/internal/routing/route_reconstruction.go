package routing

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// IsochroneRouteStep identifies one retained node in its source frontier.
type IsochroneRouteStep struct {
	FrontierIndex int
	NodeIndex     int
	Node          IsochroneNode
}

// IsochroneRoute is the chronological path from the search origin to the best
// node of the final frontier.
type IsochroneRoute struct {
	Origin      navigation.Position
	Destination navigation.Position
	StartedAt   time.Time
	FinishedAt  time.Time
	Steps       []IsochroneRouteStep
}

// Positions returns the origin followed by every reached position.
func (route IsochroneRoute) Positions() []navigation.Position {
	positions := make([]navigation.Position, 0, len(route.Steps)+1)
	positions = append(positions, route.Origin)
	for _, step := range route.Steps {
		positions = append(
			positions,
			step.Node.Candidate.Step.NextPosition,
		)
	}
	return positions
}

// ReconstructBestRoute follows parent indexes from the best final node and
// returns the selected path in chronological order.
func (search IsochroneSearch) ReconstructBestRoute() (IsochroneRoute, error) {
	if err := search.Origin.Validate(); err != nil {
		return IsochroneRoute{}, fmt.Errorf(
			"routing: reconstruction impossible: origine invalide: %w",
			err,
		)
	}
	if err := search.Destination.Validate(); err != nil {
		return IsochroneRoute{}, fmt.Errorf(
			"routing: reconstruction impossible: destination invalide: %w",
			err,
		)
	}
	if search.StartedAt.IsZero() {
		return IsochroneRoute{}, errors.New(
			"routing: reconstruction impossible: instant initial requis",
		)
	}
	if search.StepDuration <= 0 {
		return IsochroneRoute{}, errors.New(
			"routing: reconstruction impossible: durée de pas strictement positive requise",
		)
	}
	if len(search.Frontiers) == 0 {
		return IsochroneRoute{}, errors.New(
			"routing: reconstruction impossible: aucun front disponible",
		)
	}

	initialDistance, err := navigation.DistanceBetween(
		search.Origin,
		search.Destination,
	)
	if err != nil {
		return IsochroneRoute{}, fmt.Errorf(
			"routing: reconstruction impossible: distance initiale impossible: %w",
			err,
		)
	}

	expectedInstant := search.StartedAt
	for index, frontier := range search.Frontiers {
		expectedInstant = expectedInstant.Add(search.StepDuration)
		if frontier.Index != index+1 {
			return IsochroneRoute{}, fmt.Errorf(
				"routing: reconstruction impossible: indice du front %d incohérent",
				index+1,
			)
		}
		if !frontier.Instant.Equal(expectedInstant) {
			return IsochroneRoute{}, fmt.Errorf(
				"routing: reconstruction impossible: instant du front %d incohérent",
				index+1,
			)
		}
		if len(frontier.Nodes) == 0 {
			return IsochroneRoute{}, fmt.Errorf(
				"routing: reconstruction impossible: front %d vide",
				index+1,
			)
		}
	}

	steps := make([]IsochroneRouteStep, len(search.Frontiers))
	nodeIndex, _ := search.bestFinalIndex()
	for frontierIndex := len(search.Frontiers) - 1; frontierIndex >= 0; frontierIndex-- {
		frontier := search.Frontiers[frontierIndex]
		if nodeIndex < 0 || nodeIndex >= len(frontier.Nodes) {
			return IsochroneRoute{}, fmt.Errorf(
				"routing: reconstruction impossible: nœud %d absent du front %d",
				nodeIndex,
				frontierIndex+1,
			)
		}

		node := frontier.Nodes[nodeIndex]
		expectedStepPosition := search.Origin
		expectedStepInstant := search.StartedAt
		if frontierIndex > 0 {
			previousFrontier := search.Frontiers[frontierIndex-1]
			if node.ParentIndex < 0 ||
				node.ParentIndex >= len(previousFrontier.Nodes) {
				return IsochroneRoute{}, fmt.Errorf(
					"routing: reconstruction impossible: parent %d absent du front %d",
					node.ParentIndex,
					frontierIndex,
				)
			}
			parent := previousFrontier.Nodes[node.ParentIndex]
			expectedStepPosition = parent.Candidate.Step.NextPosition
			expectedStepInstant = previousFrontier.Instant
		} else if node.ParentIndex != -1 {
			return IsochroneRoute{}, errors.New(
				"routing: reconstruction impossible: le premier nœud doit avoir le parent -1",
			)
		}

		if err := validateRouteNode(
			frontierIndex,
			node,
			expectedStepPosition,
			expectedStepInstant,
			frontier.Instant,
			search.StepDuration,
			initialDistance,
			search.Destination,
		); err != nil {
			return IsochroneRoute{}, err
		}

		steps[frontierIndex] = IsochroneRouteStep{
			FrontierIndex: frontierIndex,
			NodeIndex:     nodeIndex,
			Node:          node,
		}
		nodeIndex = node.ParentIndex
	}

	return IsochroneRoute{
		Origin:      search.Origin,
		Destination: search.Destination,
		StartedAt:   search.StartedAt,
		FinishedAt:  search.Frontiers[len(search.Frontiers)-1].Instant,
		Steps:       steps,
	}, nil
}

func validateRouteNode(
	frontierIndex int,
	node IsochroneNode,
	expectedPosition navigation.Position,
	expectedInstant time.Time,
	expectedNextInstant time.Time,
	stepDuration time.Duration,
	initialDistance navigation.NauticalMiles,
	destination navigation.Position,
) error {
	number := frontierIndex + 1
	if err := node.Candidate.Heading.Validate(); err != nil {
		return fmt.Errorf(
			"routing: reconstruction impossible: cap du front %d invalide: %w",
			number,
			err,
		)
	}
	if node.Candidate.Step.Heading != node.Candidate.Heading {
		return fmt.Errorf(
			"routing: reconstruction impossible: cap du front %d incohérent",
			number,
		)
	}
	if node.Candidate.Step.Position != expectedPosition {
		return fmt.Errorf(
			"routing: reconstruction impossible: position initiale du front %d incohérente",
			number,
		)
	}
	if !node.Candidate.Step.Instant.Equal(expectedInstant) {
		return fmt.Errorf(
			"routing: reconstruction impossible: instant initial du front %d incohérent",
			number,
		)
	}
	if node.Candidate.Step.Duration != stepDuration {
		return fmt.Errorf(
			"routing: reconstruction impossible: durée du front %d incohérente",
			number,
		)
	}
	if !node.Candidate.Step.NextInstant.Equal(expectedNextInstant) {
		return fmt.Errorf(
			"routing: reconstruction impossible: instant final du front %d incohérent",
			number,
		)
	}
	if err := node.Candidate.Step.NextPosition.Validate(); err != nil {
		return fmt.Errorf(
			"routing: reconstruction impossible: position finale du front %d invalide: %w",
			number,
			err,
		)
	}
	if node.InitialDistance != initialDistance {
		return fmt.Errorf(
			"routing: reconstruction impossible: distance initiale du front %d incohérente",
			number,
		)
	}
	if err := node.RemainingDistance.Validate(); err != nil {
		return fmt.Errorf(
			"routing: reconstruction impossible: distance restante du front %d invalide: %w",
			number,
			err,
		)
	}

	remainingDistance, err := navigation.DistanceBetween(
		node.Candidate.Step.NextPosition,
		destination,
	)
	if err != nil {
		return fmt.Errorf(
			"routing: reconstruction impossible: distance restante du front %d impossible: %w",
			number,
			err,
		)
	}
	if math.Abs(
		float64(node.RemainingDistance-remainingDistance),
	) > 1e-9 {
		return fmt.Errorf(
			"routing: reconstruction impossible: distance restante du front %d incohérente",
			number,
		)
	}

	expectedProgress := DistanceProgress(
		float64(initialDistance - node.RemainingDistance),
	)
	if math.Abs(float64(node.Progress-expectedProgress)) > 1e-9 {
		return fmt.Errorf(
			"routing: reconstruction impossible: progression du front %d incohérente",
			number,
		)
	}
	return nil
}

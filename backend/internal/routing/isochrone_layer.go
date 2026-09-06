package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// IsochroneLayerInput describes one full expansion from a single origin.
type IsochroneLayerInput struct {
	Position       navigation.Position
	Destination    navigation.Position
	Instant        time.Time
	Duration       time.Duration
	Owned          equipment.EquipmentSet
	HeadingSpacing float64
}

// IsochroneLayer contains every evaluated heading, ordered by progress toward
// the destination.
type IsochroneLayer struct {
	Origin      navigation.Position
	Destination navigation.Position
	Instant     time.Time
	Duration    time.Duration
	Candidates  []CandidateProgress
}

// Best returns the candidate with the greatest progress toward the
// destination.
func (layer IsochroneLayer) Best() (CandidateProgress, bool) {
	if len(layer.Candidates) == 0 {
		return CandidateProgress{}, false
	}
	return layer.Candidates[0], true
}

// BuildIsochroneLayer generates a full heading grid, evaluates every heading
// from the same state and ranks the results by progress toward the destination.
func BuildIsochroneLayer(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input IsochroneLayerInput,
) (IsochroneLayer, error) {
	if err := input.Destination.Validate(); err != nil {
		return IsochroneLayer{}, fmt.Errorf(
			"routing: construction de la couche impossible: destination invalide: %w",
			err,
		)
	}

	headings, err := GenerateHeadingGrid(input.HeadingSpacing)
	if err != nil {
		return IsochroneLayer{}, fmt.Errorf(
			"routing: construction de la couche impossible: grille de caps invalide: %w",
			err,
		)
	}

	candidates, err := EvaluateHeadingCandidates(
		ctx,
		weatherProvider,
		boatPolar,
		HeadingCandidatesInput{
			Position: input.Position,
			Instant:  input.Instant,
			Duration: input.Duration,
			Owned:    input.Owned,
			Headings: headings,
		},
	)
	if err != nil {
		return IsochroneLayer{}, fmt.Errorf(
			"routing: construction de la couche impossible: évaluation des caps impossible: %w",
			err,
		)
	}

	ranked, err := RankCandidatesByProgress(
		input.Destination,
		candidates,
	)
	if err != nil {
		return IsochroneLayer{}, fmt.Errorf(
			"routing: construction de la couche impossible: classement impossible: %w",
			err,
		)
	}

	return IsochroneLayer{
		Origin:      input.Position,
		Destination: input.Destination,
		Instant:     input.Instant,
		Duration:    input.Duration,
		Candidates:  ranked,
	}, nil
}

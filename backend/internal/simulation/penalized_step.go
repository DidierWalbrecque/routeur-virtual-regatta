package simulation

import (
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// ApplySpeedPenaltyToStep replaces the distance and destination of an already
// evaluated step without recalculating its weather, sail or base performance.
func ApplySpeedPenaltyToStep(
	step StepResult,
	penalty *polar.ManeuverPenalty,
) (StepResult, SpeedPenaltyApplication, error) {
	application, err := ApplyManeuverSpeedPenalty(
		navigation.Knots(step.BestSail.Speed),
		step.Duration,
		penalty,
	)
	if err != nil {
		return StepResult{}, SpeedPenaltyApplication{}, fmt.Errorf(
			"simulation: correction du pas par pénalité impossible: %w",
			err,
		)
	}

	nextPosition, err := navigation.DestinationPosition(
		step.Position,
		step.Heading,
		application.Distance,
	)
	if err != nil {
		return StepResult{}, SpeedPenaltyApplication{}, fmt.Errorf(
			"simulation: correction du pas par pénalité impossible: position suivante impossible: %w",
			err,
		)
	}

	step.Distance = application.Distance
	step.NextPosition = nextPosition
	return step, application, nil
}

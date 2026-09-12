package simulation

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// SpeedPenaltyApplication describes the distance covered during one step and
// any penalty duration that must be carried to the following step.
type SpeedPenaltyApplication struct {
	Distance            navigation.NauticalMiles
	PenalizedDuration   time.Duration
	UnpenalizedDuration time.Duration
	RemainingPenalty    *polar.ManeuverPenalty
}

// ApplyManeuverSpeedPenalty integrates one already-combined speed penalty over
// a movement step. A nil penalty represents an entirely unpenalized step.
func ApplyManeuverSpeedPenalty(
	baseSpeed navigation.Knots,
	stepDuration time.Duration,
	penalty *polar.ManeuverPenalty,
) (SpeedPenaltyApplication, error) {
	if err := baseSpeed.Validate(); err != nil {
		return SpeedPenaltyApplication{}, fmt.Errorf(
			"simulation: application de pénalité impossible: vitesse invalide: %w",
			err,
		)
	}
	if stepDuration <= 0 {
		return SpeedPenaltyApplication{}, errors.New(
			"simulation: application de pénalité impossible: durée de pas strictement positive requise",
		)
	}

	if penalty == nil {
		distance, err := distanceAtSpeedForDuration(baseSpeed, stepDuration)
		if err != nil {
			return SpeedPenaltyApplication{}, err
		}
		return SpeedPenaltyApplication{
			Distance:            distance,
			UnpenalizedDuration: stepDuration,
		}, nil
	}
	if err := validateAppliedSpeedPenalty(*penalty); err != nil {
		return SpeedPenaltyApplication{}, fmt.Errorf(
			"simulation: application de pénalité impossible: %w",
			err,
		)
	}

	penalizedDuration := min(stepDuration, penalty.Duration)
	unpenalizedDuration := stepDuration - penalizedDuration
	penalizedSpeed := navigation.Knots(
		float64(baseSpeed) * float64(penalty.SpeedRatio),
	)
	penalizedDistance, err := distanceAtSpeedForDuration(
		penalizedSpeed,
		penalizedDuration,
	)
	if err != nil {
		return SpeedPenaltyApplication{}, err
	}
	unpenalizedDistance, err := distanceAtSpeedForDuration(
		baseSpeed,
		unpenalizedDuration,
	)
	if err != nil {
		return SpeedPenaltyApplication{}, err
	}

	result := SpeedPenaltyApplication{
		Distance:            penalizedDistance + unpenalizedDistance,
		PenalizedDuration:   penalizedDuration,
		UnpenalizedDuration: unpenalizedDuration,
	}
	if penalty.Duration > stepDuration {
		remaining := *penalty
		remaining.Duration -= stepDuration
		result.RemainingPenalty = &remaining
	}
	return result, nil
}

func validateAppliedSpeedPenalty(penalty polar.ManeuverPenalty) error {
	ratio := float64(penalty.SpeedRatio)
	if math.IsNaN(ratio) ||
		math.IsInf(ratio, 0) ||
		penalty.SpeedRatio < 0 ||
		penalty.SpeedRatio > 1 {
		return errors.New(
			"coefficient de vitesse fini compris entre 0 et 1 requis",
		)
	}
	if penalty.Duration <= 0 {
		return errors.New("durée de pénalité strictement positive requise")
	}
	return nil
}

func distanceAtSpeedForDuration(
	speed navigation.Knots,
	duration time.Duration,
) (navigation.NauticalMiles, error) {
	distance, err := navigation.DistanceAtConstantSpeed(
		speed,
		navigation.Seconds(duration.Seconds()),
	)
	if err != nil {
		return 0, fmt.Errorf(
			"simulation: application de pénalité impossible: calcul de distance impossible: %w",
			err,
		)
	}
	return distance, nil
}

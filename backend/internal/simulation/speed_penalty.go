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

// ManeuverSpeedPenaltyCombiner resolves simultaneous nautical and sail-change
// speed penalties. No default is provided until the in-game rule is confirmed.
type ManeuverSpeedPenaltyCombiner func(
	nautical polar.ManeuverPenalty,
	sailChange polar.ManeuverPenalty,
) (polar.ManeuverPenalty, error)

// CombinedSpeedPenalty returns the performance penalty represented by a
// transition. Simultaneous penalties require an explicit combination rule.
func (evaluation TransitionEvaluation) CombinedSpeedPenalty(
	combine ManeuverSpeedPenaltyCombiner,
) (*polar.ManeuverPenalty, error) {
	switch {
	case evaluation.NauticalPenalty == nil &&
		evaluation.SailChangePenalty == nil:
		return nil, nil
	case evaluation.NauticalPenalty != nil &&
		evaluation.SailChangePenalty == nil:
		return validatedSpeedPenaltyCopy(
			"pénalité nautique",
			evaluation.NauticalPenalty,
		)
	case evaluation.NauticalPenalty == nil &&
		evaluation.SailChangePenalty != nil:
		return validatedSpeedPenaltyCopy(
			"pénalité de changement de voile",
			evaluation.SailChangePenalty,
		)
	}

	if _, err := validatedSpeedPenaltyCopy(
		"pénalité nautique",
		evaluation.NauticalPenalty,
	); err != nil {
		return nil, err
	}
	if _, err := validatedSpeedPenaltyCopy(
		"pénalité de changement de voile",
		evaluation.SailChangePenalty,
	); err != nil {
		return nil, err
	}
	if combine == nil {
		return nil, errors.New(
			"simulation: combinaison de pénalités impossible: règle explicite requise pour des pénalités simultanées",
		)
	}

	combined, err := combine(
		*evaluation.NauticalPenalty,
		*evaluation.SailChangePenalty,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"simulation: combinaison de pénalités impossible: %w",
			err,
		)
	}
	if err := validateAppliedSpeedPenalty(combined); err != nil {
		return nil, fmt.Errorf(
			"simulation: combinaison de pénalités impossible: résultat invalide: %w",
			err,
		)
	}

	return &combined, nil
}

func validatedSpeedPenaltyCopy(
	label string,
	penalty *polar.ManeuverPenalty,
) (*polar.ManeuverPenalty, error) {
	if err := validateAppliedSpeedPenalty(*penalty); err != nil {
		return nil, fmt.Errorf(
			"simulation: combinaison de pénalités impossible: %s invalide: %w",
			label,
			err,
		)
	}
	copy := *penalty
	return &copy, nil
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

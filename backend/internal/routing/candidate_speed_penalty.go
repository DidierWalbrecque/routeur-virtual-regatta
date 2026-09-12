package routing

import (
	"errors"
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func evaluateCandidateTransitionAndPenalty(
	parent isochroneParentState,
	candidate *HeadingCandidate,
	owned equipment.EquipmentSet,
	evaluator *simulation.TransitionEvaluator,
	combine simulation.ManeuverSpeedPenaltyCombiner,
) (
	*simulation.ManeuverState,
	*simulation.TransitionEvaluation,
	*simulation.SpeedPenaltyApplication,
	error,
) {
	if candidate == nil {
		return nil, nil, nil, errors.New("candidat requis")
	}

	maneuver, err := candidateManeuver(parent, *candidate)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("manœuvre invalide: %w", err)
	}

	var transition *simulation.TransitionEvaluation
	var triggeredPenalty *polar.ManeuverPenalty
	if maneuver != nil && evaluator != nil {
		evaluation, evaluateErr := evaluator.Evaluate(
			simulation.TransitionEvaluationRequest{
				Transition: simulation.ManeuverTransition{
					PreviousTack:      parent.BoatState.Tack,
					NextTack:          candidate.Step.Tack,
					PreviousWindAngle: parent.WindAngle,
					NextWindAngle:     candidate.Step.WindAngle,
					PreviousSailID:    parent.BoatState.SailID,
					NextSailID:        candidate.Step.BestSail.SailID,
				},
				WindSpeed: polar.WindSpeed(candidate.Step.WindKnots),
				Stamina:   parent.Stamina,
				ProfessionalWinches: owned.Has(
					equipment.ProWinches,
				),
			},
		)
		if evaluateErr != nil {
			return nil, nil, nil, fmt.Errorf(
				"évaluation de transition impossible: %w",
				evaluateErr,
			)
		}
		maneuver = &evaluation.Maneuver
		transition = &evaluation

		triggeredPenalty, err = evaluation.CombinedSpeedPenalty(combine)
		if err != nil {
			return nil, nil, nil, fmt.Errorf(
				"pénalité de transition impossible: %w",
				err,
			)
		}
	}

	var activePenalty *polar.ManeuverPenalty
	if parent.HasBoatState {
		activePenalty = parent.BoatState.ActiveSpeedPenalty
	}
	stepPenalty, err := simulation.CombineSpeedPenalties(
		activePenalty,
		triggeredPenalty,
		combine,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"pénalité du pas impossible: %w",
			err,
		)
	}
	if stepPenalty == nil {
		return maneuver, transition, nil, nil
	}

	corrected, application, err := simulation.ApplySpeedPenaltyToStep(
		candidate.Step,
		stepPenalty,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"application de la pénalité au pas impossible: %w",
			err,
		)
	}
	candidate.Step = corrected
	return maneuver, transition, &application, nil
}

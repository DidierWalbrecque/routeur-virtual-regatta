package simulation

import (
	"errors"
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// ManeuverTransition contains the state needed to identify the actions between
// two navigation steps.
type ManeuverTransition struct {
	PreviousTack      navigation.Tack
	NextTack          navigation.Tack
	PreviousWindAngle navigation.TrueWindAngle
	NextWindAngle     navigation.TrueWindAngle
	PreviousSailID    polar.SailID
	NextSailID        polar.SailID
}

// DetectManeuver identifies tack, gybe and sail change without applying any
// performance or stamina penalty.
func DetectManeuver(
	transition ManeuverTransition,
) (ManeuverState, error) {
	if err := transition.PreviousTack.Validate(); err != nil {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: amure précédente invalide: %w",
			err,
		)
	}
	if err := transition.NextTack.Validate(); err != nil {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: nouvelle amure invalide: %w",
			err,
		)
	}
	if err := transition.PreviousWindAngle.Validate(); err != nil {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: TWA précédent invalide: %w",
			err,
		)
	}
	if err := transition.NextWindAngle.Validate(); err != nil {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: nouveau TWA invalide: %w",
			err,
		)
	}
	if transition.PreviousSailID <= 0 {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: identifiant de voile précédent positif requis, reçu %d",
			transition.PreviousSailID,
		)
	}
	if transition.NextSailID <= 0 {
		return ManeuverState{}, fmt.Errorf(
			"simulation: détection de manœuvre impossible: nouvel identifiant de voile positif requis, reçu %d",
			transition.NextSailID,
		)
	}

	result := ManeuverState{
		Kind: ManeuverNone,
		SailChanged: transition.PreviousSailID !=
			transition.NextSailID,
	}

	if transition.PreviousTack == navigation.TackUndefined ||
		transition.NextTack == navigation.TackUndefined ||
		transition.PreviousTack == transition.NextTack {
		return result, nil
	}

	angleSum := transition.PreviousWindAngle +
		transition.NextWindAngle
	switch {
	case angleSum < 180:
		result.Kind = ManeuverTack
	case angleSum > 180:
		result.Kind = ManeuverGybe
	default:
		return ManeuverState{}, errors.New(
			"simulation: détection de manœuvre impossible: transition ambiguë à 180 degrés",
		)
	}
	return result, nil
}

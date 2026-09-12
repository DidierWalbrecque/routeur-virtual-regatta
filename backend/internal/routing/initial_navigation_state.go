package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// InitialNavigationState contains the current heading and sail when they are
// known. The corresponding TWA and tack are derived from the initial weather.
type InitialNavigationState struct {
	Heading navigation.Heading
	SailID  polar.SailID
}

// Validate checks the independent initial navigation values.
func (state InitialNavigationState) Validate() error {
	if err := state.Heading.Validate(); err != nil {
		return fmt.Errorf(
			"routing: état nautique initial: cap invalide: %w",
			err,
		)
	}
	if state.SailID <= 0 {
		return fmt.Errorf(
			"routing: état nautique initial: identifiant de voile positif requis, reçu %d",
			state.SailID,
		)
	}
	return nil
}

func resolveInitialBoatState(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	position navigation.Position,
	instant time.Time,
	stamina simulation.StaminaState,
	owned equipment.EquipmentSet,
	navigationState InitialNavigationState,
) (simulation.BoatState, navigation.TrueWindAngle, error) {
	if err := navigationState.Validate(); err != nil {
		return simulation.BoatState{}, 0, err
	}

	effects, err := equipment.ResolveEffects(boatPolar, owned)
	if err != nil {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial: résolution des équipements impossible: %w",
			err,
		)
	}
	if !containsSailID(effects.AvailableSails, navigationState.SailID) {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial: voile %d indisponible avec les équipements possédés",
			navigationState.SailID,
		)
	}

	wind, err := weatherProvider.WindAt(ctx, position, instant)
	if err != nil {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial: vent impossible: %w",
			err,
		)
	}
	windAngle, err := navigation.TrueWindAngleFromHeading(
		navigationState.Heading,
		wind.Direction,
	)
	if err != nil {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial: TWA impossible: %w",
			err,
		)
	}
	tack, err := navigation.TackFromHeading(
		navigationState.Heading,
		wind.Direction,
	)
	if err != nil {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial: amure impossible: %w",
			err,
		)
	}

	state := simulation.BoatState{
		Position: position,
		Instant:  instant,
		Heading:  navigationState.Heading,
		SailID:   navigationState.SailID,
		Tack:     tack,
		Stamina:  stamina,
	}
	if err := state.Validate(); err != nil {
		return simulation.BoatState{}, 0, fmt.Errorf(
			"routing: état nautique initial invalide: %w",
			err,
		)
	}
	return state, windAngle, nil
}

func containsSailID(sails []polar.SailID, wanted polar.SailID) bool {
	for _, sailID := range sails {
		if sailID == wanted {
			return true
		}
	}
	return false
}

var _ = errors.New

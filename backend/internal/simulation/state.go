package simulation

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// StaminaLevel is the boat energy percentage, bounded from 0 to 100.
type StaminaLevel float64

// Validate checks that the stamina level is finite and within its bounds.
func (level StaminaLevel) Validate() error {
	value := float64(level)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errors.New("simulation: stamina finie requise")
	}
	if level < 0 || level > 100 {
		return fmt.Errorf(
			"simulation: stamina %.6f hors de l'intervalle [0, 100]",
			level,
		)
	}
	return nil
}

// StaminaState contains the energy carried from one navigation step to the
// next. Further recovery and consumption parameters remain outside this state.
type StaminaState struct {
	Level StaminaLevel
}

// Validate checks the stamina state.
func (state StaminaState) Validate() error {
	if err := state.Level.Validate(); err != nil {
		return fmt.Errorf("simulation: état de stamina invalide: %w", err)
	}
	return nil
}

// ManeuverKind identifies the nautical maneuver between two boat states.
type ManeuverKind string

const (
	ManeuverNone ManeuverKind = "none"
	ManeuverTack ManeuverKind = "tack"
	ManeuverGybe ManeuverKind = "gybe"
)

// Validate checks that the maneuver kind is known.
func (kind ManeuverKind) Validate() error {
	switch kind {
	case ManeuverNone, ManeuverTack, ManeuverGybe:
		return nil
	default:
		return fmt.Errorf("simulation: type de manœuvre inconnu %q", kind)
	}
}

// ManeuverState keeps the nautical maneuver separate from a sail change. This
// represents simultaneous actions without deciding yet how their penalties
// combine.
type ManeuverState struct {
	Kind        ManeuverKind
	SailChanged bool
}

// Validate checks the maneuver state.
func (state ManeuverState) Validate() error {
	if err := state.Kind.Validate(); err != nil {
		return fmt.Errorf("simulation: état de manœuvre invalide: %w", err)
	}
	return nil
}

// Active reports whether at least one maneuver or sail change occurred.
func (state ManeuverState) Active() bool {
	return state.Kind != ManeuverNone || state.SailChanged
}

// BoatState is the state that must be carried between route nodes once
// maneuver and stamina effects are evaluated.
type BoatState struct {
	Position navigation.Position
	Instant  time.Time
	Heading  navigation.Heading
	SailID   polar.SailID
	Tack     navigation.Tack
	Stamina  StaminaState
}

// Validate checks every independent part of a boat state.
func (state BoatState) Validate() error {
	if err := state.Position.Validate(); err != nil {
		return fmt.Errorf("simulation: état du bateau: position invalide: %w", err)
	}
	if state.Instant.IsZero() {
		return errors.New(
			"simulation: état du bateau: instant requis",
		)
	}
	if err := state.Heading.Validate(); err != nil {
		return fmt.Errorf("simulation: état du bateau: cap invalide: %w", err)
	}
	if state.SailID <= 0 {
		return fmt.Errorf(
			"simulation: état du bateau: identifiant de voile positif requis, reçu %d",
			state.SailID,
		)
	}
	if err := state.Tack.Validate(); err != nil {
		return fmt.Errorf("simulation: état du bateau: amure invalide: %w", err)
	}
	if err := state.Stamina.Validate(); err != nil {
		return fmt.Errorf("simulation: état du bateau: %w", err)
	}
	return nil
}

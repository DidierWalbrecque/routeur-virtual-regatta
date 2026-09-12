package simulation

import "fmt"

// StaminaConsumptionRules contains the base energy cost of each independent
// action. Wind and current-stamina coefficients are applied in later steps.
type StaminaConsumptionRules struct {
	Tack       StaminaLevel
	Gybe       StaminaLevel
	SailChange StaminaLevel
}

// DefaultStaminaConsumptionRules returns the currently confirmed Virtual
// Regatta reference costs.
func DefaultStaminaConsumptionRules() StaminaConsumptionRules {
	return StaminaConsumptionRules{
		Tack:       10,
		Gybe:       10,
		SailChange: 20,
	}
}

// Validate checks that every configured cost is a valid stamina percentage.
func (rules StaminaConsumptionRules) Validate() error {
	values := []struct {
		name  string
		value StaminaLevel
	}{
		{name: "virement", value: rules.Tack},
		{name: "empannage", value: rules.Gybe},
		{name: "changement de voile", value: rules.SailChange},
	}
	for _, item := range values {
		if err := item.value.Validate(); err != nil {
			return fmt.Errorf(
				"simulation: coût de stamina pour %s invalide: %w",
				item.name,
				err,
			)
		}
	}
	return nil
}

// StaminaConsumption keeps the nautical-maneuver cost separate from the
// sail-change cost. Their combination rule is deliberately left configurable.
type StaminaConsumption struct {
	Nautical   StaminaLevel
	SailChange StaminaLevel
}

// Validate checks both independent stamina-consumption components.
func (consumption StaminaConsumption) Validate() error {
	if err := consumption.Nautical.Validate(); err != nil {
		return fmt.Errorf(
			"simulation: consommation nautique invalide: %w",
			err,
		)
	}
	if err := consumption.SailChange.Validate(); err != nil {
		return fmt.Errorf(
			"simulation: consommation de changement de voile invalide: %w",
			err,
		)
	}
	return nil
}

// ConsumptionFor returns the base energy consumption of a detected maneuver.
// Simultaneous nautical and sail-change costs remain separate because their
// exact in-game combination rule is not confirmed yet.
func (rules StaminaConsumptionRules) ConsumptionFor(
	maneuver ManeuverState,
) (StaminaConsumption, error) {
	if err := rules.Validate(); err != nil {
		return StaminaConsumption{}, fmt.Errorf(
			"simulation: calcul de consommation impossible: %w",
			err,
		)
	}
	if err := maneuver.Validate(); err != nil {
		return StaminaConsumption{}, fmt.Errorf(
			"simulation: calcul de consommation impossible: %w",
			err,
		)
	}

	result := StaminaConsumption{}
	switch maneuver.Kind {
	case ManeuverNone:
	case ManeuverTack:
		result.Nautical = rules.Tack
	case ManeuverGybe:
		result.Nautical = rules.Gybe
	}
	if maneuver.SailChanged {
		result.SailChange = rules.SailChange
	}
	return result, nil
}

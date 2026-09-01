package equipment

import "fmt"

// Combination décrit une configuration d'équipements respectant le plafond.
type Combination struct {
	Active                    EquipmentSet
	ToPurchase                EquipmentSet
	Cost                      CostBreakdown
	PurchaseCost              Credits
	AdditionalPersonalCredits Credits
}

// GenerateAffordableCombinations génère toutes les configurations contenant
// les équipements déjà possédés, puis conserve celles dont le coût total ne
// dépasse pas le plafond de la course.
func GenerateAffordableCombinations(
	configuration RaceConfiguration,
) ([]Combination, error) {
	if err := configuration.Validate(); err != nil {
		return nil, fmt.Errorf(
			"equipment: génération des combinaisons impossible: %w",
			err,
		)
	}

	ownedCost, err := BreakdownAt(
		configuration.Budget.Category,
		configuration.Owned,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"equipment: calcul des équipements possédés impossible: %w",
			err,
		)
	}
	if ownedCost.TotalCost > configuration.Budget.TotalLimit {
		return nil, fmt.Errorf(
			"equipment: les équipements possédés coûtent %d crédits et dépassent le plafond total de %d crédits",
			ownedCost.TotalCost,
			configuration.Budget.TotalLimit,
		)
	}

	combinations := make([]Combination, 0, int(allEquipments)+1)
	for mask := EquipmentSet(0); mask <= allEquipments; mask++ {
		active := mask
		if active&configuration.Owned != configuration.Owned {
			continue
		}

		cost, costErr := BreakdownAt(
			configuration.Budget.Category,
			active,
		)
		if costErr != nil {
			return nil, fmt.Errorf(
				"equipment: calcul de la combinaison %08b impossible: %w",
				active,
				costErr,
			)
		}
		if cost.TotalCost > configuration.Budget.TotalLimit {
			continue
		}

		combinations = append(combinations, Combination{
			Active:                    active,
			ToPurchase:                active &^ configuration.Owned,
			Cost:                      cost,
			PurchaseCost:              cost.TotalCost - ownedCost.TotalCost,
			AdditionalPersonalCredits: cost.PersonalCredits - ownedCost.PersonalCredits,
		})
	}

	return combinations, nil
}

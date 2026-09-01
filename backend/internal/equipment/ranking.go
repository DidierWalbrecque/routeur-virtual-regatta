package equipment

import (
	"fmt"
	"sort"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// RankedCombination associe une combinaison abordable à sa meilleure voile et
// à son gain de vitesse par rapport aux équipements déjà possédés.
type RankedCombination struct {
	Combination
	BestSail  polar.SailPerformance
	SpeedGain polar.BoatSpeed
}

// RankAffordableAt classe les combinaisons abordables pour un vent instantané.
// La vitesse la plus élevée est prioritaire. À vitesse égale, le classement
// préfère le moins de crédits personnels supplémentaires, puis le coût total.
func RankAffordableAt(
	boatPolar *polar.Polar,
	configuration RaceConfiguration,
	windSpeed polar.WindSpeed,
	windAngle polar.TrueWindAngle,
) ([]RankedCombination, error) {
	if boatPolar == nil {
		return nil, fmt.Errorf(
			"equipment: classement instantané impossible: la polaire est nil",
		)
	}

	combinations, err := GenerateAffordableCombinations(configuration)
	if err != nil {
		return nil, fmt.Errorf(
			"equipment: classement instantané impossible: %w",
			err,
		)
	}

	baseline, found := findCombinationByActive(
		combinations,
		configuration.Owned,
	)
	if !found {
		return nil, fmt.Errorf(
			"equipment: classement instantané impossible: configuration possédée introuvable",
		)
	}

	baselinePerformance, err := performanceAt(
		boatPolar,
		baseline,
		windSpeed,
		windAngle,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"equipment: calcul de la configuration possédée impossible: %w",
			err,
		)
	}

	ranked := make([]RankedCombination, 0, len(combinations))
	for _, combination := range combinations {
		performance, performanceErr := performanceAt(
			boatPolar,
			combination,
			windSpeed,
			windAngle,
		)
		if performanceErr != nil {
			return nil, fmt.Errorf(
				"equipment: calcul de la combinaison %08b impossible: %w",
				combination.Active,
				performanceErr,
			)
		}

		ranked = append(ranked, RankedCombination{
			Combination: combination,
			BestSail:    performance,
			SpeedGain:   performance.Speed - baselinePerformance.Speed,
		})
	}

	sort.SliceStable(ranked, func(left int, right int) bool {
		leftResult := ranked[left]
		rightResult := ranked[right]

		if leftResult.BestSail.Speed != rightResult.BestSail.Speed {
			return leftResult.BestSail.Speed > rightResult.BestSail.Speed
		}
		if leftResult.AdditionalPersonalCredits !=
			rightResult.AdditionalPersonalCredits {
			return leftResult.AdditionalPersonalCredits <
				rightResult.AdditionalPersonalCredits
		}
		if leftResult.Cost.TotalCost != rightResult.Cost.TotalCost {
			return leftResult.Cost.TotalCost < rightResult.Cost.TotalCost
		}
		return leftResult.Active < rightResult.Active
	})

	return ranked, nil
}

func performanceAt(
	boatPolar *polar.Polar,
	combination Combination,
	windSpeed polar.WindSpeed,
	windAngle polar.TrueWindAngle,
) (polar.SailPerformance, error) {
	effects, err := ResolveEffects(boatPolar, combination.Active)
	if err != nil {
		return polar.SailPerformance{}, err
	}

	return boatPolar.BestEffectiveSailAt(
		windSpeed,
		windAngle,
		effects.AvailableSails,
		effects.SpeedOptions,
	)
}

func findCombinationByActive(
	combinations []Combination,
	active EquipmentSet,
) (Combination, bool) {
	for _, combination := range combinations {
		if combination.Active == active {
			return combination, true
		}
	}
	return Combination{}, false
}

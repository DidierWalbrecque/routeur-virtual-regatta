package equipment

import (
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// PurchaseEvaluation décrit l'effet instantané de l'achat d'une seule option.
type PurchaseEvaluation struct {
	Equipment Equipment
	Combination
	BestSail  polar.SailPerformance
	SpeedGain polar.BoatSpeed
}

// RankNextPurchasesAt classe les achats unitaires encore abordables pour un
// vent instantané. Les équipements déjà possédés sont conservés et exclus des
// achats proposés.
func RankNextPurchasesAt(
	boatPolar *polar.Polar,
	configuration RaceConfiguration,
	windSpeed polar.WindSpeed,
	windAngle polar.TrueWindAngle,
) ([]PurchaseEvaluation, error) {
	ranked, err := RankAffordableAt(
		boatPolar,
		configuration,
		windSpeed,
		windAngle,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"equipment: classement des prochains achats impossible: %w",
			err,
		)
	}

	evaluations := make([]PurchaseEvaluation, 0, len(All()))
	for _, candidate := range ranked {
		item := Equipment(candidate.ToPurchase)
		if item.Validate() != nil {
			// La configuration de référence n'achète rien. Les autres
			// configurations peuvent contenir plusieurs achats.
			continue
		}

		evaluations = append(evaluations, PurchaseEvaluation{
			Equipment:   item,
			Combination: candidate.Combination,
			BestSail:    candidate.BestSail,
			SpeedGain:   candidate.SpeedGain,
		})
	}

	return evaluations, nil
}

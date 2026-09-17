package routing

import "github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"

// practicalRobustness compares nominal progress with the worst progress among
// the weather scenarios. A scenario that improves progress does not create a
// loss. Without scenarios, the nominal simulation is robust by definition.
func practicalRobustness(
	nominal DistanceProgress,
	scenarios []PracticalHeadingWeatherScenario,
	maximumLoss navigation.NauticalMiles,
) (worstCase DistanceProgress, loss navigation.NauticalMiles, robust bool) {
	worstCase = nominal
	for _, scenario := range scenarios {
		if scenario.Progress < worstCase {
			worstCase = scenario.Progress
		}
	}
	loss = navigation.NauticalMiles(float64(nominal - worstCase))
	if loss < 0 {
		loss = 0
	}
	return worstCase, loss, loss <= maximumLoss
}

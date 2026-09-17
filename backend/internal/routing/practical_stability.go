package routing

import (
	"math"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// practicalStabilityScore measures how regular the effective boat speed is
// across a practical-heading simulation. It returns a value in [0, 1], where
// 1 means that every step has the same speed. It deliberately does not model
// weather perturbations; those belong to a later practical-routing phase.
func practicalStabilityScore(result simulation.Result) float64 {
	if len(result.Steps) == 0 {
		return 0
	}

	var total float64
	for _, step := range result.Steps {
		total += float64(step.BestSail.Speed)
	}
	mean := total / float64(len(result.Steps))
	if mean == 0 {
		return 1
	}

	var squaredDifferences float64
	for _, step := range result.Steps {
		difference := float64(step.BestSail.Speed) - mean
		squaredDifferences += difference * difference
	}
	coefficientOfVariation := math.Sqrt(
		squaredDifferences/float64(len(result.Steps)),
	) / mean
	return math.Max(0, 1-coefficientOfVariation)
}

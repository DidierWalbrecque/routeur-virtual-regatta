package routing

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

const (
	// maxRefinedMaxima limits how many promising directions are refined for
	// one parent: typically the best heading on each tack, plus one spare.
	maxRefinedMaxima = 3
	// minRefinableSpacing is the grid spacing below which refinement would not
	// bring anything: integer headings are already at most 1.5° away.
	minRefinableSpacing = 3.0
)

// headingRefinement enables the search of integer headings around the best
// headings of the regular grid. waypoints holds, for each course gate index,
// the moving target used for progress: the nearest point on the gate line, or
// the final destination once every gate has been crossed.
type headingRefinement struct {
	spacing   float64
	waypoints []courseWaypoint
}

// refinementSteps returns the successive integer steps, in degrees, used to
// converge on the best heading. Their sum is at least half the grid spacing so
// an optimum sitting between two grid headings stays reachable.
func refinementSteps(spacing float64) []int {
	if spacing < minRefinableSpacing {
		return nil
	}
	step := int(math.Ceil(spacing / 3))
	steps := []int{}
	for step >= 1 {
		steps = append(steps, step)
		if step == 1 {
			break
		}
		next := int(math.Round(float64(step) * 0.4))
		if next < 1 {
			next = 1
		}
		step = next
	}
	// Pour les très petits pas, répéter le palier de 1° jusqu'à pouvoir
	// atteindre la moitié du pas.
	reach := 0
	for _, value := range steps {
		reach += value
	}
	for float64(reach) < spacing/2 {
		steps = append(steps, 1)
		reach++
	}
	return steps
}

func normalizeIntegerHeading(value float64) navigation.Heading {
	degrees := math.Mod(math.Round(value), 360)
	if degrees < 0 {
		degrees += 360
	}
	return navigation.Heading(degrees)
}

// refineHeadingCandidates adds candidates at integer headings around the local
// maxima of progress toward the target. The input candidates are returned
// unchanged, followed by the new ones, so a disabled or ineffective refinement
// is a no-op. The weather provider should cache the parent's single wind
// lookup, which every refinement call reuses.
func refineHeadingCandidates(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	base HeadingCandidatesInput,
	candidates []HeadingCandidate,
	target navigation.Position,
	spacing float64,
) ([]HeadingCandidate, error) {
	steps := refinementSteps(spacing)
	if len(steps) == 0 || len(candidates) < 3 {
		return candidates, nil
	}

	initial, err := navigation.DistanceBetween(base.Position, target)
	if err != nil {
		return nil, fmt.Errorf("routing: distance initiale impossible pour l'affinage des caps: %w", err)
	}
	progressOf := func(candidate HeadingCandidate) (float64, error) {
		remaining, distanceErr := navigation.DistanceBetween(candidate.Step.NextPosition, target)
		if distanceErr != nil {
			return 0, distanceErr
		}
		return float64(initial - remaining), nil
	}

	scores := make(map[navigation.Heading]float64, len(candidates)*2)
	progress := make([]float64, len(candidates))
	for index, candidate := range candidates {
		value, scoreErr := progressOf(candidate)
		if scoreErr != nil {
			return nil, fmt.Errorf("routing: progression impossible pour l'affinage des caps: %w", scoreErr)
		}
		progress[index] = value
		scores[candidate.Heading] = value
	}

	// Local maxima around the heading circle; the candidates follow the grid
	// order, so neighbours are the adjacent indexes.
	maxima := make([]int, 0, len(candidates))
	for index := range candidates {
		previous := progress[(index+len(candidates)-1)%len(candidates)]
		next := progress[(index+1)%len(candidates)]
		if progress[index] >= previous && progress[index] > next {
			maxima = append(maxima, index)
		}
	}
	sort.SliceStable(maxima, func(left, right int) bool {
		return progress[maxima[left]] > progress[maxima[right]]
	})
	if len(maxima) > maxRefinedMaxima {
		maxima = maxima[:maxRefinedMaxima]
	}

	result := candidates
	for _, index := range maxima {
		center := normalizeIntegerHeading(float64(candidates[index].Heading))
		centerScore, known := scores[center]
		if !known {
			evaluated, evalErr := evaluateRefinementHeadings(ctx, weatherProvider, boatPolar, base, []navigation.Heading{center})
			if evalErr != nil {
				return nil, evalErr
			}
			for _, candidate := range evaluated {
				value, scoreErr := progressOf(candidate)
				if scoreErr != nil {
					return nil, scoreErr
				}
				scores[candidate.Heading] = value
				result = append(result, candidate)
			}
			centerScore = scores[center]
		}

		for _, step := range steps {
			wanted := []navigation.Heading{
				normalizeIntegerHeading(float64(center) - float64(step)),
				normalizeIntegerHeading(float64(center) + float64(step)),
			}
			missing := make([]navigation.Heading, 0, 2)
			for _, heading := range wanted {
				if _, seen := scores[heading]; !seen {
					missing = append(missing, heading)
					scores[heading] = math.Inf(-1) // réservé pendant l'évaluation
				}
			}
			if len(missing) > 0 {
				evaluated, evalErr := evaluateRefinementHeadings(ctx, weatherProvider, boatPolar, base, missing)
				if evalErr != nil {
					return nil, evalErr
				}
				for _, candidate := range evaluated {
					value, scoreErr := progressOf(candidate)
					if scoreErr != nil {
						return nil, scoreErr
					}
					scores[candidate.Heading] = value
					result = append(result, candidate)
				}
			}
			for _, heading := range wanted {
				if value := scores[heading]; value > centerScore {
					center, centerScore = heading, value
				}
			}
		}
	}
	return result, nil
}

func evaluateRefinementHeadings(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	base HeadingCandidatesInput,
	headings []navigation.Heading,
) ([]HeadingCandidate, error) {
	input := base
	input.Headings = headings
	return EvaluateHeadingCandidates(ctx, weatherProvider, boatPolar, input)
}

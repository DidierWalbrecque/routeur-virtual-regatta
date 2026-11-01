package routing

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type parentHeadingResult struct {
	index      int
	candidates []HeadingCandidate
	err        error
}

// evaluateParentHeadingCandidates evaluates independent parent positions in
// parallel. Results are returned in parent order so the expansion and all
// constraints remain deterministic and sequential.
func evaluateParentHeadingCandidates(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	parents []isochroneParentState,
	instant time.Time,
	duration time.Duration,
	owned equipment.EquipmentSet,
	effects *equipment.Effects,
	headings []navigation.Heading,
	refine *headingRefinement,
) ([][]HeadingCandidate, error) {
	if len(parents) == 0 {
		return nil, nil
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(parents) {
		workerCount = len(parents)
	}

	jobs := make(chan int, len(parents))
	results := make(chan parentHeadingResult, len(parents))
	for index := range parents {
		jobs <- index
	}
	close(jobs)

	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					results <- parentHeadingResult{index: index, err: err}
					continue
				}
				input := HeadingCandidatesInput{
					Position: parents[index].Position,
					Instant:  instant,
					Duration: duration,
					Owned:    owned,
					Effects:  effects,
					Headings: headings,
				}
				provider := weatherProvider
				if refine != nil {
					// Un seul relevé de vent par parent, partagé avec l'affinage.
					provider = &headingWeatherCache{
						provider: weatherProvider,
						position: input.Position,
						instant:  input.Instant,
					}
				}
				candidates, err := EvaluateHeadingCandidates(ctx, provider, boatPolar, input)
				if err == nil && refine != nil {
					nextGate := parents[index].CourseProgress.NextGateIndex
					if nextGate >= 0 && nextGate < len(refine.targets) {
						candidates, err = refineHeadingCandidates(
							ctx, provider, boatPolar, input, candidates,
							refine.targets[nextGate], refine.spacing,
						)
					}
				}
				results <- parentHeadingResult{index: index, candidates: candidates, err: err}
			}
		}()
	}
	workers.Wait()
	close(results)

	ordered := make([]parentHeadingResult, 0, len(parents))
	for result := range results {
		ordered = append(ordered, result)
	}
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].index < ordered[right].index
	})

	allCandidates := make([][]HeadingCandidate, len(parents))
	for _, result := range ordered {
		if result.err != nil {
			return nil, fmt.Errorf("développement du parent %d impossible: %w", result.index+1, result.err)
		}
		allCandidates[result.index] = result.candidates
	}
	return allCandidates, nil
}

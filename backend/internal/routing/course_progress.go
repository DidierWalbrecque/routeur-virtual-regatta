package routing

import (
	"fmt"
)

// CourseProgress identifies the next mandatory gate to cross.
type CourseProgress struct {
	NextGateIndex int
}

// Validate checks that the progress belongs to the configured gate sequence.
func (progress CourseProgress) Validate(gates []CourseGate) error {
	for index, gate := range gates {
		if err := gate.Validate(); err != nil {
			return fmt.Errorf(
				"routing: porte obligatoire %d invalide: %w",
				index+1,
				err,
			)
		}
	}
	if progress.NextGateIndex < 0 ||
		progress.NextGateIndex > len(gates) {
		return fmt.Errorf(
			"routing: prochaine porte %d hors de la séquence de %d portes",
			progress.NextGateIndex,
			len(gates),
		)
	}
	return nil
}

// Complete reports whether every mandatory gate has been crossed.
func (progress CourseProgress) Complete(
	gates []CourseGate,
) (bool, error) {
	if err := progress.Validate(gates); err != nil {
		return false, err
	}
	return progress.NextGateIndex == len(gates), nil
}

// Advance returns the progress reached by segment. Several consecutive gates
// may be counted when they are crossed in the configured order during the same
// segment. A missing gate or a crossing in the wrong direction stops progress
// without invalidating the route.
func (progress CourseProgress) Advance(
	gates []CourseGate,
	segment RouteSegment,
) (CourseProgress, error) {
	if err := progress.Validate(gates); err != nil {
		return CourseProgress{}, err
	}
	if err := segment.Validate(); err != nil {
		return CourseProgress{}, fmt.Errorf(
			"routing: progression de parcours impossible: segment invalide: %w",
			err,
		)
	}

	next := progress.NextGateIndex
	previousParameter := -1.0
	for next < len(gates) {
		evaluation, err := gates[next].evaluateCrossing(segment)
		if err != nil {
			return CourseProgress{}, fmt.Errorf(
				"routing: évaluation de la porte obligatoire %d impossible: %w",
				next+1,
				err,
			)
		}
		if evaluation.status != GateCrossedAllowed {
			break
		}
		if evaluation.routeParameter <=
			previousParameter+gateGeometryEpsilon {
			break
		}
		previousParameter = evaluation.routeParameter
		next++
	}
	return CourseProgress{NextGateIndex: next}, nil
}

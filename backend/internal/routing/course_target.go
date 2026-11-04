package routing

import (
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// Midpoint returns the geographic center of the gate along the shortest
// longitude interval, including gates that straddle the antimeridian.
func (gate CourseGate) Midpoint() (navigation.Position, error) {
	if err := gate.Validate(); err != nil {
		return navigation.Position{}, err
	}

	longitudeDelta, err := navigation.ShortestLongitudeDelta(
		gate.FirstMark.Longitude,
		gate.SecondMark.Longitude,
	)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: centre de la porte %q impossible: %w",
			gate.Name,
			err,
		)
	}
	longitude, err := navigation.NormalizeLongitude(
		float64(gate.FirstMark.Longitude) + longitudeDelta/2,
	)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: centre de la porte %q impossible: %w",
			gate.Name,
			err,
		)
	}

	midpoint := navigation.Position{
		Latitude: navigation.Latitude(
			(float64(gate.FirstMark.Latitude) +
				float64(gate.SecondMark.Latitude)) / 2,
		),
		Longitude: longitude,
	}
	if err := midpoint.Validate(); err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: centre de la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}
	return midpoint, nil
}

// Target returns the center of the next mandatory gate. Once every gate has
// been crossed, the final destination becomes the active target.
func (progress CourseProgress) Target(
	gates []CourseGate,
	destination navigation.Position,
) (navigation.Position, error) {
	if err := progress.Validate(gates); err != nil {
		return navigation.Position{}, err
	}
	if err := destination.Validate(); err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: destination finale invalide: %w",
			err,
		)
	}
	if progress.NextGateIndex == len(gates) {
		return destination, nil
	}

	target, err := gates[progress.NextGateIndex].Midpoint()
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: cible de la porte obligatoire %d impossible: %w",
			progress.NextGateIndex+1,
			err,
		)
	}
	return target, nil
}

// courseWaypoint is the moving target used to measure course progress: the
// nearest point on an uncrossed gate's line, or the fixed final destination
// once every gate has been crossed. Aiming at the nearest point on the gate
// line rather than its centre lets the router pick the fastest crossing point
// instead of being pulled toward the middle of the gate.
type courseWaypoint struct {
	gate        *CourseGate
	destination navigation.Position
}

// nearestFrom returns the position aimed at from the given position.
func (waypoint courseWaypoint) nearestFrom(
	position navigation.Position,
) (navigation.Position, error) {
	if waypoint.gate != nil {
		return waypoint.gate.ClosestPoint(position)
	}
	return waypoint.destination, nil
}

// distanceFrom returns the distance to the waypoint from the given position,
// together with the aimed-at position used for that distance.
func (waypoint courseWaypoint) distanceFrom(
	position navigation.Position,
) (navigation.NauticalMiles, navigation.Position, error) {
	target, err := waypoint.nearestFrom(position)
	if err != nil {
		return 0, navigation.Position{}, err
	}
	distance, err := navigation.DistanceBetween(position, target)
	if err != nil {
		return 0, navigation.Position{}, err
	}
	return distance, target, nil
}

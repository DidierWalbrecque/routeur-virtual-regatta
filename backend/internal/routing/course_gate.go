package routing

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

const gateGeometryEpsilon = 1e-12

// GateDirection describes the authorized side change relative to the oriented
// line from FirstMark to SecondMark.
type GateDirection uint8

const (
	// GateEitherDirection accepts a crossing in both directions.
	GateEitherDirection GateDirection = iota
	// GateRightToLeft requires the boat to move from the right side of the
	// oriented gate line to its left side.
	GateRightToLeft
	// GateLeftToRight requires the boat to move from the left side of the
	// oriented gate line to its right side.
	GateLeftToRight
)

// Validate checks that the direction is supported.
func (direction GateDirection) Validate() error {
	switch direction {
	case GateEitherDirection, GateRightToLeft, GateLeftToRight:
		return nil
	default:
		return fmt.Errorf(
			"routing: sens de franchissement de porte %d invalide",
			direction,
		)
	}
}

// GateCrossingStatus describes how one route segment interacts with a gate.
type GateCrossingStatus uint8

const (
	GateNotCrossed GateCrossingStatus = iota
	GateCrossedAllowed
	GateCrossedWrongDirection
)

// CourseGate is an oriented line segment between two course marks.
type CourseGate struct {
	Name       string
	FirstMark  navigation.Position
	SecondMark navigation.Position
	Direction  GateDirection
}

// Validate checks the gate name, marks and crossing direction.
func (gate CourseGate) Validate() error {
	if strings.TrimSpace(gate.Name) == "" {
		return errors.New("routing: nom de porte requis")
	}
	if err := gate.FirstMark.Validate(); err != nil {
		return fmt.Errorf(
			"routing: première marque de la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}
	if err := gate.SecondMark.Validate(); err != nil {
		return fmt.Errorf(
			"routing: seconde marque de la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}
	if err := gate.Direction.Validate(); err != nil {
		return fmt.Errorf("routing: porte %q invalide: %w", gate.Name, err)
	}

	second, err := projectGatePosition(gate.FirstMark, gate.SecondMark)
	if err != nil {
		return fmt.Errorf(
			"routing: géométrie de la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}
	if math.Hypot(second.x, second.y) <= gateGeometryEpsilon {
		return fmt.Errorf(
			"routing: les deux marques de la porte %q doivent être distinctes",
			gate.Name,
		)
	}
	return nil
}

// Evaluate reports whether segment crosses the finite gate line and whether
// that crossing follows the configured direction. Merely touching the gate at
// the beginning or end of a segment is not considered a crossing.
func (gate CourseGate) Evaluate(
	segment RouteSegment,
) (GateCrossingStatus, error) {
	evaluation, err := gate.evaluateCrossing(segment)
	if err != nil {
		return GateNotCrossed, err
	}
	return evaluation.status, nil
}

type gateCrossingEvaluation struct {
	status         GateCrossingStatus
	routeParameter float64
}

func (gate CourseGate) evaluateCrossing(
	segment RouteSegment,
) (gateCrossingEvaluation, error) {
	if err := gate.Validate(); err != nil {
		return gateCrossingEvaluation{}, err
	}
	if err := segment.Validate(); err != nil {
		return gateCrossingEvaluation{}, fmt.Errorf(
			"routing: segment soumis à la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}

	gateEnd, err := projectGatePosition(gate.FirstMark, gate.SecondMark)
	if err != nil {
		return gateCrossingEvaluation{}, err
	}
	routeStart, err := projectGatePosition(gate.FirstMark, segment.From)
	if err != nil {
		return gateCrossingEvaluation{}, err
	}
	routeEnd, err := projectGatePosition(gate.FirstMark, segment.To)
	if err != nil {
		return gateCrossingEvaluation{}, err
	}

	startSide := crossGatePoints(gateEnd, routeStart)
	endSide := crossGatePoints(gateEnd, routeEnd)
	if math.Abs(startSide) <= gateGeometryEpsilon ||
		math.Abs(endSide) <= gateGeometryEpsilon ||
		startSide*endSide >= 0 {
		return gateCrossingEvaluation{status: GateNotCrossed}, nil
	}

	routeVector := gatePoint{
		x: routeEnd.x - routeStart.x,
		y: routeEnd.y - routeStart.y,
	}
	denominator := crossGatePoints(routeVector, gateEnd)
	if math.Abs(denominator) <= gateGeometryEpsilon {
		return gateCrossingEvaluation{status: GateNotCrossed}, nil
	}
	fromRouteToGate := gatePoint{
		x: -routeStart.x,
		y: -routeStart.y,
	}
	routeParameter := crossGatePoints(fromRouteToGate, gateEnd) /
		denominator
	gateParameter := crossGatePoints(fromRouteToGate, routeVector) /
		denominator
	if routeParameter <= gateGeometryEpsilon ||
		routeParameter >= 1-gateGeometryEpsilon ||
		gateParameter < -gateGeometryEpsilon ||
		gateParameter > 1+gateGeometryEpsilon {
		return gateCrossingEvaluation{status: GateNotCrossed}, nil
	}

	actualDirection := GateLeftToRight
	if startSide < 0 && endSide > 0 {
		actualDirection = GateRightToLeft
	}
	status := GateCrossedWrongDirection
	if gate.Direction == GateEitherDirection ||
		gate.Direction == actualDirection {
		status = GateCrossedAllowed
	}
	return gateCrossingEvaluation{
		status:         status,
		routeParameter: routeParameter,
	}, nil
}

type gatePoint struct {
	x float64
	y float64
}

func projectGatePosition(
	reference navigation.Position,
	position navigation.Position,
) (gatePoint, error) {
	longitudeDelta, err := navigation.ShortestLongitudeDelta(
		reference.Longitude,
		position.Longitude,
	)
	if err != nil {
		return gatePoint{}, fmt.Errorf(
			"routing: projection de longitude impossible: %w",
			err,
		)
	}
	referenceLatitudeRadians := float64(reference.Latitude) * math.Pi / 180
	return gatePoint{
		x: longitudeDelta * math.Cos(referenceLatitudeRadians),
		y: float64(position.Latitude - reference.Latitude),
	}, nil
}

func crossGatePoints(first, second gatePoint) float64 {
	return first.x*second.y - first.y*second.x
}

// ClosestPoint returns the point of the finite gate segment nearest to the
// given position, in the gate's local flat frame. It lets the router measure
// progress toward the gate line itself rather than toward its centre, so every
// valid crossing point is treated as equally good. The gate is assumed already
// validated by the caller.
func (gate CourseGate) ClosestPoint(
	from navigation.Position,
) (navigation.Position, error) {
	gateEnd, err := projectGatePosition(gate.FirstMark, gate.SecondMark)
	if err != nil {
		return navigation.Position{}, err
	}
	fromVector, err := projectGatePosition(gate.FirstMark, from)
	if err != nil {
		return navigation.Position{}, err
	}
	lengthSquared := gateEnd.x*gateEnd.x + gateEnd.y*gateEnd.y
	parameter := 0.0
	if lengthSquared > gateGeometryEpsilon {
		parameter = (fromVector.x*gateEnd.x + fromVector.y*gateEnd.y) /
			lengthSquared
		if parameter < 0 {
			parameter = 0
		}
		if parameter > 1 {
			parameter = 1
		}
	}
	closest := gatePoint{x: parameter * gateEnd.x, y: parameter * gateEnd.y}
	referenceLatitudeRadians := float64(gate.FirstMark.Latitude) * math.Pi / 180
	cosLatitude := math.Cos(referenceLatitudeRadians)
	offsetLongitude := 0.0
	if math.Abs(cosLatitude) > gateGeometryEpsilon {
		offsetLongitude = closest.x / cosLatitude
	}
	longitude, err := navigation.NormalizeLongitude(
		float64(gate.FirstMark.Longitude) + offsetLongitude,
	)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: point le plus proche de la porte %q impossible: %w",
			gate.Name,
			err,
		)
	}
	point := navigation.Position{
		Latitude: navigation.Latitude(
			float64(gate.FirstMark.Latitude) + closest.y,
		),
		Longitude: longitude,
	}
	if err := point.Validate(); err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routing: point le plus proche de la porte %q invalide: %w",
			gate.Name,
			err,
		)
	}
	return point, nil
}

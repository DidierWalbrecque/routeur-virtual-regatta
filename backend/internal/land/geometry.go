package land

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

const geometryEpsilon = 1e-12

// Allows reports whether the segment stays entirely outside terrestrial
// polygons. Touching a coastline is considered forbidden.
func (constraint *GeoJSONConstraint) Allows(
	ctx context.Context,
	segment routing.RouteSegment,
) (bool, error) {
	if constraint == nil {
		return false, errors.New("land: contrainte GeoJSON requise")
	}
	if ctx == nil {
		return false, errors.New("land: contexte requis")
	}
	if err := segment.Validate(); err != nil {
		return false, fmt.Errorf("land: segment invalide: %w", err)
	}

	start := point{
		longitude: float64(segment.From.Longitude),
		latitude:  float64(segment.From.Latitude),
	}
	longitudeDelta, err := navigation.ShortestLongitudeDelta(
		segment.From.Longitude,
		segment.To.Longitude,
	)
	if err != nil {
		return false, fmt.Errorf(
			"land: longitude du segment impossible: %w",
			err,
		)
	}
	end := point{
		longitude: start.longitude + longitudeDelta,
		latitude:  float64(segment.To.Latitude),
	}
	referenceLongitude := (start.longitude + end.longitude) / 2

	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf(
			"land: vérification interrompue: %w",
			err,
		)
	}
	margin := float64(constraint.coastalMargin)
	for _, index := range constraint.candidatePolygonIndices(
		start,
		end,
		margin,
	) {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf(
				"land: vérification interrompue au polygone %d: %w",
				index+1,
				err,
			)
		}
		value := constraint.polygons[index]
		forbidden, err := segmentForbiddenByPolygon(
			start,
			end,
			value,
			referenceLongitude,
			margin,
		)
		if err != nil {
			return false, err
		}
		if forbidden {
			return false, nil
		}
	}
	return true, nil
}

func segmentForbiddenByPolygon(
	start,
	end point,
	value polygon,
	referenceLongitude,
	margin float64,
) (bool, error) {
	if !polygonMayAffectSegment(value, start, end, referenceLongitude, margin) {
		return false, nil
	}
	rings := value.unwrappedRings
	polygonReference := value.referenceLongitude
	if len(rings) == 0 {
		rings = make([][]point, len(value.rings))
		polygonReference = referenceLongitude
		if len(value.rings) > 0 && len(value.rings[0]) > 0 {
			polygonReference = value.rings[0][0].longitude
		}
		for index, ring := range value.rings {
			rings[index] = unwrapRing(ring, polygonReference)
		}
	}
	shift := 360 * math.Round((referenceLongitude-polygonReference)/360)
	localStart := start
	localStart.longitude -= shift
	localEnd := end
	localEnd.longitude -= shift
	localReferenceLongitude := referenceLongitude - shift
	referenceLatitude := (localStart.latitude + localEnd.latitude) / 2
	longitudeScale := math.Cos(referenceLatitude * math.Pi / 180)

	if value.grid != nil && len(value.unwrappedRings) > 0 {
		forbidden, handled := segmentForbiddenByIndexedPolygon(
			localStart,
			localEnd,
			value,
			localReferenceLongitude,
			referenceLatitude,
			longitudeScale,
			margin,
		)
		if handled {
			return forbidden, nil
		}
	}

	for _, ring := range rings {
		if segmentIntersectsRing(localStart, localEnd, ring) {
			return true, nil
		}
	}
	if pointInPolygon(localStart, rings) || pointInPolygon(localEnd, rings) {
		return true, nil
	}
	if margin <= 0 {
		return false, nil
	}

	projectedStart := projectPointWithScale(localStart, localReferenceLongitude, referenceLatitude, longitudeScale)
	projectedEnd := projectPointWithScale(localEnd, localReferenceLongitude, referenceLatitude, longitudeScale)
	distances := newMarginDistances()
	for _, ring := range rings {
		for index := 1; index < len(ring); index++ {
			edgeStart := projectPointWithScale(ring[index-1], localReferenceLongitude, referenceLatitude, longitudeScale)
			edgeEnd := projectPointWithScale(ring[index], localReferenceLongitude, referenceLatitude, longitudeScale)
			distances.add(projectedStart, projectedEnd, edgeStart, edgeEnd)
		}
	}
	return distances.forbidden(margin), nil
}

// marginDistances gathers, for one polygon, the distances from the coast that
// decide whether a segment respects the coastal margin.
type marginDistances struct {
	segment float64
	start   float64
	end     float64
}

func newMarginDistances() marginDistances {
	return marginDistances{segment: math.Inf(1), start: math.Inf(1), end: math.Inf(1)}
}

func (distances *marginDistances) add(start, end, edgeStart, edgeEnd projectedPoint) {
	distances.segment = math.Min(distances.segment, projectedSegmentDistance(start, end, edgeStart, edgeEnd))
	distances.start = math.Min(distances.start, projectedPointToSegmentDistance(start, edgeStart, edgeEnd))
	distances.end = math.Min(distances.end, projectedPointToSegmentDistance(end, edgeStart, edgeEnd))
}

// marginEscapeTolerance absorbs rounding when comparing two distances, in
// nautical miles.
const marginEscapeTolerance = 1e-9

// forbidden reports whether the segment breaks the coastal margin. A boat
// that is already inside the margin (a race start or finish close to the
// shore) may still leave it: its segment is accepted when it ends farther from
// the coast than it starts and never comes closer than its starting point.
func (distances marginDistances) forbidden(margin float64) bool {
	if distances.segment > margin+geometryEpsilon {
		return false
	}
	if distances.start > margin+geometryEpsilon {
		return true
	}
	movesAway := distances.end > distances.start+marginEscapeTolerance
	neverCloser := distances.segment >= distances.start-marginEscapeTolerance
	return !(movesAway && neverCloser)
}

// segmentForbiddenByIndexedPolygon gives the same answer as the exhaustive
// checks of segmentForbiddenByPolygon while only visiting the polygon edges
// close to the segment. handled is false when the query is too large for the
// grid, in which case the caller falls back to the exhaustive scan.
func segmentForbiddenByIndexedPolygon(
	localStart,
	localEnd point,
	value polygon,
	localReferenceLongitude,
	referenceLatitude,
	longitudeScale,
	margin float64,
) (forbidden, handled bool) {
	latitudeExpansion := margin / 60
	longitudeExpansion := 0.0
	if margin > 0 {
		if longitudeScale < 1e-6 {
			return false, false
		}
		longitudeExpansion = margin / (60 * longitudeScale)
	}
	// Small relative slack so rounding can never exclude an edge that the
	// exhaustive scan would have accepted.
	latitudeExpansion = latitudeExpansion*(1+1e-9) + edgeGridPad
	longitudeExpansion = longitudeExpansion*(1+1e-9) + edgeGridPad

	var buffer [256]edgeRef
	edges, ok := value.grid.collect(
		math.Min(localStart.longitude, localEnd.longitude)-longitudeExpansion,
		math.Max(localStart.longitude, localEnd.longitude)+longitudeExpansion,
		math.Min(localStart.latitude, localEnd.latitude)-latitudeExpansion,
		math.Max(localStart.latitude, localEnd.latitude)+latitudeExpansion,
		buffer[:0],
	)
	if !ok {
		return false, false
	}

	rings := value.unwrappedRings
	for _, edge := range edges {
		if edge.index == 0 {
			continue
		}
		ring := rings[edge.ring]
		if segmentsIntersect(localStart, localEnd, ring[edge.index-1], ring[edge.index]) {
			return true, true
		}
	}
	if value.grid.pointInPolygon(localStart, rings) ||
		value.grid.pointInPolygon(localEnd, rings) {
		return true, true
	}
	if margin <= 0 {
		return false, true
	}

	projectedStart := projectPointWithScale(localStart, localReferenceLongitude, referenceLatitude, longitudeScale)
	projectedEnd := projectPointWithScale(localEnd, localReferenceLongitude, referenceLatitude, longitudeScale)
	// Edges farther than the margin from the segment cannot change the answer:
	// they are farther than the margin from its start and its end too.
	distances := newMarginDistances()
	for _, edge := range edges {
		if edge.index == 0 {
			continue
		}
		ring := rings[edge.ring]
		edgeStart := projectPointWithScale(ring[edge.index-1], localReferenceLongitude, referenceLatitude, longitudeScale)
		edgeEnd := projectPointWithScale(ring[edge.index], localReferenceLongitude, referenceLatitude, longitudeScale)
		distances.add(projectedStart, projectedEnd, edgeStart, edgeEnd)
	}
	return distances.forbidden(margin), true
}

func projectPointWithScale(
	value point,
	referenceLongitude,
	referenceLatitude,
	longitudeScale float64,
) projectedPoint {
	return projectedPoint{
		x: (value.longitude - referenceLongitude) * 60 * longitudeScale,
		y: (value.latitude - referenceLatitude) * 60,
	}
}

func polygonMayAffectSegment(
	value polygon,
	start,
	end point,
	referenceLongitude,
	margin float64,
) bool {
	minLongitude, maxLongitude, minLatitude, maxLatitude, ok := polygonBounds(value)
	if !ok {
		return false
	}

	latitudeMargin := margin / 60
	segmentMinLatitude := math.Min(start.latitude, end.latitude) - latitudeMargin
	segmentMaxLatitude := math.Max(start.latitude, end.latitude) + latitudeMargin
	if segmentMaxLatitude < minLatitude || segmentMinLatitude > maxLatitude {
		return false
	}

	maximumAbsoluteLatitude := math.Max(
		math.Abs(start.latitude),
		math.Abs(end.latitude),
	)
	longitudeScale := math.Cos(maximumAbsoluteLatitude * math.Pi / 180)
	longitudeMargin := margin / 60
	if longitudeScale > 1e-9 {
		longitudeMargin = margin / (60 * longitudeScale)
	} else {
		longitudeMargin = 180
	}
	if longitudeMargin > 180 {
		longitudeMargin = 180
	}

	segmentMinLongitude := math.Min(start.longitude, end.longitude) - longitudeMargin
	segmentMaxLongitude := math.Max(start.longitude, end.longitude) + longitudeMargin
	polygonReference := value.referenceLongitude
	if !value.boundsReady && len(value.rings) > 0 && len(value.rings[0]) > 0 {
		polygonReference = value.rings[0][0].longitude
	}
	shift := 360 * math.Round((referenceLongitude-polygonReference)/360)
	minLongitude += shift
	maxLongitude += shift
	return segmentMaxLongitude >= minLongitude && segmentMinLongitude <= maxLongitude
}

func segmentTouchesPolygon(
	start,
	end point,
	value polygon,
	referenceLongitude float64,
) bool {
	rings := make([][]point, len(value.rings))
	for index, ring := range value.rings {
		rings[index] = unwrapRing(ring, referenceLongitude)
		if segmentIntersectsRing(start, end, rings[index]) {
			return true
		}
	}
	return pointInPolygon(start, rings) || pointInPolygon(end, rings)
}

func unwrapRing(ring []point, referenceLongitude float64) []point {
	result := make([]point, len(ring))
	if len(ring) == 0 {
		return result
	}
	firstLongitude := nearestLongitude(
		ring[0].longitude,
		referenceLongitude,
	)
	result[0] = point{
		longitude: firstLongitude,
		latitude:  ring[0].latitude,
	}
	for index := 1; index < len(ring); index++ {
		delta := shortestLongitudeDeltaValues(
			ring[index-1].longitude,
			ring[index].longitude,
		)
		result[index] = point{
			longitude: result[index-1].longitude + delta,
			latitude:  ring[index].latitude,
		}
	}
	return result
}

func nearestLongitude(longitude, reference float64) float64 {
	return longitude + 360*math.Round((reference-longitude)/360)
}

func shortestLongitudeDeltaValues(from, to float64) float64 {
	delta := math.Mod(to-from+540, 360) - 180
	if delta == -180 && to-from > 0 {
		return 180
	}
	return delta
}

func segmentIntersectsRing(start, end point, ring []point) bool {
	for index := 1; index < len(ring); index++ {
		if segmentsIntersect(start, end, ring[index-1], ring[index]) {
			return true
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d point) bool {
	orientation1 := orientation(a, b, c)
	orientation2 := orientation(a, b, d)
	orientation3 := orientation(c, d, a)
	orientation4 := orientation(c, d, b)

	if oppositeSigns(orientation1, orientation2) &&
		oppositeSigns(orientation3, orientation4) {
		return true
	}
	return math.Abs(orientation1) <= geometryEpsilon &&
		pointOnSegment(c, a, b) ||
		math.Abs(orientation2) <= geometryEpsilon &&
			pointOnSegment(d, a, b) ||
		math.Abs(orientation3) <= geometryEpsilon &&
			pointOnSegment(a, c, d) ||
		math.Abs(orientation4) <= geometryEpsilon &&
			pointOnSegment(b, c, d)
}

type projectedPoint struct {
	x float64
	y float64
}

func segmentDistanceToPolygon(
	start,
	end point,
	value polygon,
	referenceLongitude float64,
) float64 {
	referenceLatitude := (start.latitude + end.latitude) / 2
	projectedStart := projectPoint(
		start,
		referenceLongitude,
		referenceLatitude,
	)
	projectedEnd := projectPoint(
		end,
		referenceLongitude,
		referenceLatitude,
	)
	minimum := math.Inf(1)
	for _, ring := range value.rings {
		unwrapped := unwrapRing(ring, referenceLongitude)
		for index := 1; index < len(unwrapped); index++ {
			edgeStart := projectPoint(
				unwrapped[index-1],
				referenceLongitude,
				referenceLatitude,
			)
			edgeEnd := projectPoint(
				unwrapped[index],
				referenceLongitude,
				referenceLatitude,
			)
			minimum = math.Min(
				minimum,
				projectedSegmentDistance(
					projectedStart,
					projectedEnd,
					edgeStart,
					edgeEnd,
				),
			)
		}
	}
	return minimum
}

func projectPoint(
	value point,
	referenceLongitude,
	referenceLatitude float64,
) projectedPoint {
	longitudeScale := math.Cos(referenceLatitude * math.Pi / 180)
	return projectedPoint{
		x: (value.longitude - referenceLongitude) * 60 * longitudeScale,
		y: (value.latitude - referenceLatitude) * 60,
	}
}

func projectedSegmentDistance(
	a,
	b,
	c,
	d projectedPoint,
) float64 {
	return math.Min(
		math.Min(
			projectedPointToSegmentDistance(a, c, d),
			projectedPointToSegmentDistance(b, c, d),
		),
		math.Min(
			projectedPointToSegmentDistance(c, a, b),
			projectedPointToSegmentDistance(d, a, b),
		),
	)
}

func projectedPointToSegmentDistance(
	value,
	start,
	end projectedPoint,
) float64 {
	deltaX := end.x - start.x
	deltaY := end.y - start.y
	lengthSquared := deltaX*deltaX + deltaY*deltaY
	if lengthSquared == 0 {
		return math.Hypot(value.x-start.x, value.y-start.y)
	}
	position := ((value.x-start.x)*deltaX +
		(value.y-start.y)*deltaY) / lengthSquared
	position = math.Max(0, math.Min(1, position))
	closestX := start.x + position*deltaX
	closestY := start.y + position*deltaY
	return math.Hypot(value.x-closestX, value.y-closestY)
}

func orientation(a, b, c point) float64 {
	return (b.longitude-a.longitude)*(c.latitude-a.latitude) -
		(b.latitude-a.latitude)*(c.longitude-a.longitude)
}

func oppositeSigns(left, right float64) bool {
	return left > geometryEpsilon && right < -geometryEpsilon ||
		left < -geometryEpsilon && right > geometryEpsilon
}

func pointOnSegment(value, start, end point) bool {
	return value.longitude >= math.Min(start.longitude, end.longitude)-geometryEpsilon &&
		value.longitude <= math.Max(start.longitude, end.longitude)+geometryEpsilon &&
		value.latitude >= math.Min(start.latitude, end.latitude)-geometryEpsilon &&
		value.latitude <= math.Max(start.latitude, end.latitude)+geometryEpsilon
}

func pointInPolygon(value point, rings [][]point) bool {
	if len(rings) == 0 {
		return false
	}
	outer := pointInRing(value, rings[0])
	if outer == ringOutside {
		return false
	}
	if outer == ringBoundary {
		return true
	}
	for _, hole := range rings[1:] {
		location := pointInRing(value, hole)
		if location == ringBoundary {
			return true
		}
		if location == ringInside {
			return false
		}
	}
	return true
}

type ringLocation uint8

const (
	ringOutside ringLocation = iota
	ringInside
	ringBoundary
)

func pointInRing(value point, ring []point) ringLocation {
	inside := false
	for index, previous := 0, len(ring)-1; index < len(ring); index++ {
		current := ring[index]
		before := ring[previous]
		if math.Abs(orientation(before, current, value)) <= geometryEpsilon &&
			pointOnSegment(value, before, current) {
			return ringBoundary
		}
		crosses := (current.latitude > value.latitude) !=
			(before.latitude > value.latitude)
		if crosses {
			longitude := (before.longitude-current.longitude)*
				(value.latitude-current.latitude)/
				(before.latitude-current.latitude) +
				current.longitude
			if value.longitude < longitude {
				inside = !inside
			}
		}
		previous = index
	}
	if inside {
		return ringInside
	}
	return ringOutside
}

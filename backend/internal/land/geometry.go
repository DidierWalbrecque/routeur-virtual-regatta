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
	for _, index := range constraint.candidatePolygonIndices(start, end) {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf(
				"land: vérification interrompue au polygone %d: %w",
				index+1,
				err,
			)
		}
		if segmentTouchesPolygon(
			start,
			end,
			constraint.polygons[index],
			referenceLongitude,
		) {
			return false, nil
		}
	}
	return true, nil
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

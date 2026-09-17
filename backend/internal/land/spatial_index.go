package land

import (
	"math"
	"sort"
)

const spatialIndexCellDegrees = 5.0

type spatialIndexCell struct {
	latitude  int
	longitude int
}

type polygonSpatialIndex map[spatialIndexCell][]int

func newPolygonSpatialIndex(polygons []polygon) polygonSpatialIndex {
	if len(polygons) == 0 {
		return nil
	}
	index := make(polygonSpatialIndex)
	for polygonIndex, value := range polygons {
		minLongitude, maxLongitude, minLatitude, maxLatitude, ok :=
			polygonBounds(value)
		if !ok {
			continue
		}
		for _, cell := range cellsForBounds(
			minLongitude,
			maxLongitude,
			minLatitude,
			maxLatitude,
		) {
			index[cell] = append(index[cell], polygonIndex)
		}
	}
	return index
}

func (constraint *GeoJSONConstraint) candidatePolygonIndices(
	start,
	end point,
	marginNauticalMiles float64,
) []int {
	if constraint.index == nil {
		result := make([]int, len(constraint.polygons))
		for index := range constraint.polygons {
			result[index] = index
		}
		return result
	}

	minLongitude := math.Min(start.longitude, end.longitude)
	maxLongitude := math.Max(start.longitude, end.longitude)
	minLatitude := math.Min(start.latitude, end.latitude)
	maxLatitude := math.Max(start.latitude, end.latitude)

	latitudeExpansion := marginNauticalMiles / 60
	maximumAbsoluteLatitude := math.Max(
		math.Abs(minLatitude-latitudeExpansion),
		math.Abs(maxLatitude+latitudeExpansion),
	)
	if maximumAbsoluteLatitude > 89.999999 {
		maximumAbsoluteLatitude = 89.999999
	}
	longitudeScale := math.Cos(maximumAbsoluteLatitude * math.Pi / 180)
	longitudeExpansion := marginNauticalMiles / (60 * longitudeScale)
	if longitudeExpansion > 180 {
		longitudeExpansion = 180
	}
	minLongitude -= longitudeExpansion
	maxLongitude += longitudeExpansion
	minLatitude -= latitudeExpansion
	maxLatitude += latitudeExpansion

	seen := make(map[int]struct{})
	for _, cell := range cellsForBounds(
		minLongitude,
		maxLongitude,
		minLatitude,
		maxLatitude,
	) {
		for _, polygonIndex := range constraint.index[cell] {
			seen[polygonIndex] = struct{}{}
		}
	}
	result := make([]int, 0, len(seen))
	for polygonIndex := range seen {
		result = append(result, polygonIndex)
	}
	sort.Ints(result)
	return result
}

func polygonBounds(
	value polygon,
) (float64, float64, float64, float64, bool) {
	if len(value.rings) == 0 || len(value.rings[0]) == 0 {
		return 0, 0, 0, 0, false
	}
	referenceLongitude := value.rings[0][0].longitude
	minLongitude := math.Inf(1)
	maxLongitude := math.Inf(-1)
	minLatitude := math.Inf(1)
	maxLatitude := math.Inf(-1)

	for _, ring := range value.rings {
		for _, current := range unwrapRing(ring, referenceLongitude) {
			minLongitude = math.Min(minLongitude, current.longitude)
			maxLongitude = math.Max(maxLongitude, current.longitude)
			minLatitude = math.Min(minLatitude, current.latitude)
			maxLatitude = math.Max(maxLatitude, current.latitude)
		}
	}
	return minLongitude, maxLongitude, minLatitude, maxLatitude, true
}

func cellsForBounds(
	minLongitude,
	maxLongitude,
	minLatitude,
	maxLatitude float64,
) []spatialIndexCell {
	minLatitudeCell := latitudeCell(minLatitude)
	maxLatitudeCell := latitudeCell(maxLatitude)
	minLongitudeCell := rawLongitudeCell(minLongitude)
	maxLongitudeCell := rawLongitudeCell(maxLongitude)

	result := make(
		[]spatialIndexCell,
		0,
		(maxLatitudeCell-minLatitudeCell+1)*
			(maxLongitudeCell-minLongitudeCell+1),
	)
	seen := make(map[spatialIndexCell]struct{})
	for latitude := minLatitudeCell; latitude <= maxLatitudeCell; latitude++ {
		for longitude := minLongitudeCell; longitude <= maxLongitudeCell; longitude++ {
			cell := spatialIndexCell{
				latitude:  latitude,
				longitude: normalizeLongitudeCell(longitude),
			}
			if _, exists := seen[cell]; exists {
				continue
			}
			seen[cell] = struct{}{}
			result = append(result, cell)
		}
	}
	return result
}

func latitudeCell(latitude float64) int {
	cell := int(math.Floor((latitude + 90) / spatialIndexCellDegrees))
	maximum := int(180/spatialIndexCellDegrees) - 1
	if cell < 0 {
		return 0
	}
	if cell > maximum {
		return maximum
	}
	return cell
}

func rawLongitudeCell(longitude float64) int {
	return int(math.Floor((longitude + 180) / spatialIndexCellDegrees))
}

func normalizeLongitudeCell(cell int) int {
	count := int(360 / spatialIndexCellDegrees)
	cell %= count
	if cell < 0 {
		cell += count
	}
	return cell
}

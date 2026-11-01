package land

import (
	"math"
	"sort"
)

const (
	// edgeGridCellDegrees is the size of one cell of the per-polygon edge grid.
	edgeGridCellDegrees = 0.5
	// edgeGridMinEdges is the number of edges below which a polygon is simply
	// scanned: building a grid would not pay for itself.
	edgeGridMinEdges = 64
	// edgeGridMaxQueryCells bounds the cells visited for one query. Larger
	// queries fall back to the exhaustive scan.
	edgeGridMaxQueryCells = 4096
	// edgeGridPad keeps edges registered in every cell they can touch, even
	// when the geometry tolerances (geometryEpsilon) apply.
	edgeGridPad = 1e-9
)

// edgeRef identifies the edge ending at ring[index]; its start is
// ring[index-1], or the last vertex when index is 0 (closing edge).
type edgeRef struct {
	ring  int32
	index int32
}

// edgeGrid indexes the edges of one polygon, in the polygon's own unwrapped
// longitude frame. It is immutable after construction, so it can be shared by
// concurrent route calculations.
type edgeGrid struct {
	cells map[int64][]edgeRef
	// rows lists, for each latitude cell, the sorted populated longitude cells.
	rows                       map[int][]int
	minCellX, maxCellX         int
	minCellY, maxCellY         int
	totalEdges, populatedCells int
}

func gridCell(value float64) int {
	return int(math.Floor(value / edgeGridCellDegrees))
}

func gridKey(x, y int) int64 {
	return int64(x)<<32 ^ int64(uint32(int32(y)))
}

// buildEdgeGrid returns nil when the polygon is too small to need a grid.
func buildEdgeGrid(rings [][]point) *edgeGrid {
	total := 0
	for _, ring := range rings {
		total += len(ring)
	}
	if total < edgeGridMinEdges {
		return nil
	}

	grid := &edgeGrid{
		cells:    make(map[int64][]edgeRef, total/2),
		minCellX: math.MaxInt,
		minCellY: math.MaxInt,
		maxCellX: math.MinInt,
		maxCellY: math.MinInt,
	}
	for ringIndex, ring := range rings {
		count := len(ring)
		for index := 0; index < count; index++ {
			previous := (index - 1 + count) % count
			a, b := ring[previous], ring[index]
			x0 := gridCell(math.Min(a.longitude, b.longitude) - edgeGridPad)
			x1 := gridCell(math.Max(a.longitude, b.longitude) + edgeGridPad)
			y0 := gridCell(math.Min(a.latitude, b.latitude) - edgeGridPad)
			y1 := gridCell(math.Max(a.latitude, b.latitude) + edgeGridPad)
			ref := edgeRef{ring: int32(ringIndex), index: int32(index)}
			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					key := gridKey(x, y)
					grid.cells[key] = append(grid.cells[key], ref)
				}
			}
			grid.minCellX = min(grid.minCellX, x0)
			grid.maxCellX = max(grid.maxCellX, x1)
			grid.minCellY = min(grid.minCellY, y0)
			grid.maxCellY = max(grid.maxCellY, y1)
			grid.totalEdges++
		}
	}

	grid.rows = make(map[int][]int)
	for key := range grid.cells {
		x := int(key >> 32)
		y := int(int32(uint32(key)))
		grid.rows[y] = append(grid.rows[y], x)
	}
	for _, cells := range grid.rows {
		sort.Ints(cells)
	}
	grid.populatedCells = len(grid.cells)
	return grid
}

// collect appends every edge registered in cells overlapping the box. It
// reports false when the box is too large to be worth querying.
func (grid *edgeGrid) collect(
	minLongitude,
	maxLongitude,
	minLatitude,
	maxLatitude float64,
	destination []edgeRef,
) ([]edgeRef, bool) {
	x0 := max(gridCell(minLongitude-edgeGridPad), grid.minCellX)
	x1 := min(gridCell(maxLongitude+edgeGridPad), grid.maxCellX)
	y0 := max(gridCell(minLatitude-edgeGridPad), grid.minCellY)
	y1 := min(gridCell(maxLatitude+edgeGridPad), grid.maxCellY)
	if x0 > x1 || y0 > y1 {
		return destination, true
	}
	if (x1-x0+1)*(y1-y0+1) > edgeGridMaxQueryCells {
		return destination, false
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			destination = append(destination, grid.cells[gridKey(x, y)]...)
		}
	}
	return destination, true
}

// pointInPolygon returns exactly what the exhaustive pointInPolygon returns
// for the same rings, but only visits the edges that can influence the
// result: those near the point (boundary test) and those crossing the
// eastward ray from the point (parity test).
func (grid *edgeGrid) pointInPolygon(value point, rings [][]point) bool {
	if len(rings) == 0 {
		return false
	}

	var parityStorage, boundaryStorage [8]bool
	parity := parityStorage[:0]
	boundary := boundaryStorage[:0]
	if len(rings) <= len(parityStorage) {
		parity = parityStorage[:len(rings)]
		boundary = boundaryStorage[:len(rings)]
	} else {
		parity = make([]bool, len(rings))
		boundary = make([]bool, len(rings))
	}

	valueCellX := gridCell(value.longitude)
	valueCellY := gridCell(value.latitude)
	if valueCellY >= grid.minCellY && valueCellY <= grid.maxCellY {
		// Boundary test: only edges registered in the point's own cell can
		// be within geometryEpsilon of it.
		for _, ref := range grid.cells[gridKey(valueCellX, valueCellY)] {
			ring := rings[ref.ring]
			count := len(ring)
			index := int(ref.index)
			before := ring[(index-1+count)%count]
			current := ring[index]
			if math.Abs(orientation(before, current, value)) <= geometryEpsilon &&
				pointOnSegment(value, before, current) {
				boundary[ref.ring] = true
			}
		}

		// Parity test along the eastward ray. A crossing is counted only in
		// the cell that contains its longitude so edges spanning several
		// cells are counted once.
		cells := grid.rows[valueCellY]
		start := sort.SearchInts(cells, valueCellX)
		for _, cellX := range cells[start:] {
			for _, ref := range grid.cells[gridKey(cellX, valueCellY)] {
				ring := rings[ref.ring]
				count := len(ring)
				index := int(ref.index)
				before := ring[(index-1+count)%count]
				current := ring[index]
				crosses := (current.latitude > value.latitude) !=
					(before.latitude > value.latitude)
				if !crosses {
					continue
				}
				longitude := (before.longitude-current.longitude)*
					(value.latitude-current.latitude)/
					(before.latitude-current.latitude) +
					current.longitude
				if value.longitude < longitude && gridCell(longitude) == cellX {
					parity[ref.ring] = !parity[ref.ring]
				}
			}
		}
	}

	location := func(ring int) ringLocation {
		if boundary[ring] {
			return ringBoundary
		}
		if parity[ring] {
			return ringInside
		}
		return ringOutside
	}
	outer := location(0)
	if outer == ringOutside {
		return false
	}
	if outer == ringBoundary {
		return true
	}
	for hole := 1; hole < len(rings); hole++ {
		switch location(hole) {
		case ringBoundary:
			return true
		case ringInside:
			return false
		}
	}
	return true
}

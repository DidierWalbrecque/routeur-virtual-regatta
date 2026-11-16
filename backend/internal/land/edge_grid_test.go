package land

import (
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
)

// referenceSegmentForbiddenByPolygon is the exhaustive implementation that
// existed before the edge grid; it is the oracle for the indexed version.
func referenceSegmentForbiddenByPolygon(
	start,
	end point,
	value polygon,
	referenceLongitude,
	margin float64,
) bool {
	if !polygonMayAffectSegment(value, start, end, referenceLongitude, margin) {
		return false
	}
	rings := value.unwrappedRings
	polygonReference := value.referenceLongitude
	shift := 360 * math.Round((referenceLongitude-polygonReference)/360)
	localStart := start
	localStart.longitude -= shift
	localEnd := end
	localEnd.longitude -= shift
	localReferenceLongitude := referenceLongitude - shift
	for _, ring := range rings {
		if segmentIntersectsRing(localStart, localEnd, ring) {
			return true
		}
	}
	if pointInPolygon(localStart, rings) || pointInPolygon(localEnd, rings) {
		return true
	}
	if margin <= 0 {
		return false
	}
	referenceLatitude := (localStart.latitude + localEnd.latitude) / 2
	projectedStart := projectPoint(localStart, localReferenceLongitude, referenceLatitude)
	projectedEnd := projectPoint(localEnd, localReferenceLongitude, referenceLatitude)
	distances := newMarginDistances()
	for _, ring := range rings {
		for index := 1; index < len(ring); index++ {
			edgeStart := projectPoint(ring[index-1], localReferenceLongitude, referenceLatitude)
			edgeEnd := projectPoint(ring[index], localReferenceLongitude, referenceLatitude)
			distances.add(projectedStart, projectedEnd, edgeStart, edgeEnd)
		}
	}
	return distances.forbidden(margin)
}

func TestIndexedSegmentMatchesExhaustiveScanOnNaturalEarth(t *testing.T) {
	const path = "../../../data/land/natural-earth-10m-land.geojson"
	if content, err := os.ReadFile(path); err != nil || strings.HasPrefix(string(content), "version https://git-lfs") {
		t.Skip("jeu de données Natural Earth absent")
	}
	constraint, err := LoadGeoJSONFile(path)
	if err != nil {
		t.Fatal(err)
	}
	indexed := 0
	for _, value := range constraint.polygons {
		if value.grid != nil {
			indexed++
		}
	}
	if indexed == 0 {
		t.Fatal("aucun polygone indexé par la grille d'arêtes")
	}

	// Regions: Biscay, Gibraltar, Danish straits, high latitudes, ±180°,
	// Aegean, Caribbean, Antarctic coast.
	regions := [][4]float64{
		{42, 48, -10, -1},
		{35, 37, -7, -4},
		{54, 58, 8, 13},
		{66, 80, -30, 40},
		{-20, 20, 170, -170},
		{35, 41, 22, 28},
		{10, 20, -85, -60},
		{-75, -60, 160, -160},
		{-5, 5, 95, 105},
		{64, 71, 170, -170},
	}
	random := rand.New(rand.NewSource(7))
	mismatches := 0
	checked := 0
	for _, region := range regions {
		for iteration := 0; iteration < 80; iteration++ {
			latitude := region[0] + random.Float64()*(region[1]-region[0])
			span := region[3] - region[2]
			if span < 0 {
				span += 360
			}
			longitude := region[2] + random.Float64()*span
			if longitude > 180 {
				longitude -= 360
			}
			length := []float64{0.02, 0.1, 0.5, 2}[random.Intn(4)]
			bearing := random.Float64() * 2 * math.Pi
			endLatitude := math.Max(-89, math.Min(89, latitude+length*math.Cos(bearing)))
			endLongitude := longitude + length*math.Sin(bearing)/math.Max(0.05, math.Cos(latitude*math.Pi/180))
			if endLongitude > 180 {
				endLongitude -= 360
			}
			if endLongitude < -180 {
				endLongitude += 360
			}
			start := point{longitude: longitude, latitude: latitude}
			delta := endLongitude - longitude
			delta = math.Mod(delta+540, 360) - 180
			end := point{longitude: longitude + delta, latitude: endLatitude}
			reference := (start.longitude + end.longitude) / 2

			for _, margin := range []float64{0, 2} {
				for index, value := range constraint.polygons {
					want := referenceSegmentForbiddenByPolygon(start, end, value, reference, margin)
					got, err := segmentForbiddenByPolygon(start, end, value, reference, margin)
					if err != nil {
						t.Fatal(err)
					}
					checked++
					if got != want {
						mismatches++
						if mismatches <= 10 {
							t.Errorf("polygone %d marge %v segment %+v→%+v: indexé=%v exhaustif=%v",
								index, margin, start, end, got, want)
						}
					}
				}
			}
		}
	}
	t.Logf("%d comparaisons, %d polygones indexés, %d écarts", checked, indexed, mismatches)
	if mismatches > 0 {
		t.Fatalf("%d écarts entre la grille et le balayage exhaustif", mismatches)
	}
}

func syntheticRing(centerLongitude, centerLatitude, radius float64, count int, wobble float64) []point {
	ring := make([]point, count+1)
	for index := 0; index < count; index++ {
		angle := 2 * math.Pi * float64(index) / float64(count)
		scale := radius * (1 + wobble*math.Sin(7*angle))
		longitude := centerLongitude + scale*math.Cos(angle)
		if longitude > 180 {
			longitude -= 360
		}
		if longitude < -180 {
			longitude += 360
		}
		ring[index] = point{longitude: longitude, latitude: centerLatitude + scale*math.Sin(angle)}
	}
	ring[count] = ring[0]
	return ring
}

// TestIndexedSegmentMatchesExhaustiveScanOnSyntheticPolygons does not need
// the Natural Earth dataset, so it always runs in CI.
func TestIndexedSegmentMatchesExhaustiveScanOnSyntheticPolygons(t *testing.T) {
	polygons := []polygon{
		{rings: [][]point{
			syntheticRing(2, 45, 1.5, 120, 0.3),
			syntheticRing(2, 45, 0.4, 70, 0.1),
		}},
		{rings: [][]point{syntheticRing(179.2, 10, 1.5, 120, 0.3)}},
		{rings: [][]point{syntheticRing(-179.5, -30, 2, 90, 0.2)}},
	}
	newPolygonSpatialIndex(polygons)
	for index, value := range polygons {
		if value.grid == nil {
			t.Fatalf("le polygone %d n'a pas de grille d'arêtes", index)
		}
	}

	random := rand.New(rand.NewSource(11))
	centers := [][2]float64{{2, 45}, {179.2, 10}, {-179.5, -30}}
	checked, forbiddenCount := 0, 0
	for iteration := 0; iteration < 4000; iteration++ {
		center := centers[iteration%len(centers)]
		longitude := center[0] + (random.Float64()-0.5)*7
		latitude := center[1] + (random.Float64()-0.5)*7
		length := []float64{0.01, 0.1, 0.6, 3}[random.Intn(4)]
		bearing := random.Float64() * 2 * math.Pi
		if longitude > 180 {
			longitude -= 360
		}
		if longitude < -180 {
			longitude += 360
		}
		start := point{longitude: longitude, latitude: latitude}
		end := point{
			longitude: longitude + length*math.Sin(bearing),
			latitude:  latitude + length*math.Cos(bearing),
		}
		reference := (start.longitude + end.longitude) / 2
		for _, margin := range []float64{0, 2} {
			for index, value := range polygons {
				want := referenceSegmentForbiddenByPolygon(start, end, value, reference, margin)
				got, err := segmentForbiddenByPolygon(start, end, value, reference, margin)
				if err != nil {
					t.Fatal(err)
				}
				checked++
				if want {
					forbiddenCount++
				}
				if got != want {
					t.Fatalf("polygone %d marge %v segment %+v→%+v: indexé=%v exhaustif=%v",
						index, margin, start, end, got, want)
				}
			}
		}
	}
	if forbiddenCount == 0 || forbiddenCount == checked {
		t.Fatalf("jeu de segments non discriminant: %d interdits sur %d", forbiddenCount, checked)
	}
}

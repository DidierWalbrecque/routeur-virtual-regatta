package routing

import (
	"fmt"
	"math"
	"sort"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// Contour returns a closed convex display envelope of retained positions.
// This is an approximation, not a navigable area: it can bridge land and
// unreachable concavities. sectors is retained for API compatibility.
func (frontier IsochroneFrontier) Contour(origin navigation.Position, sectors int) ([]navigation.Position, error) {
	if sectors < 8 {
		return nil, fmt.Errorf("routing: au moins 8 secteurs requis")
	}
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	points := make([]navigation.Position, 0, len(frontier.Nodes))
	for _, node := range frontier.Nodes {
		position := node.Candidate.Step.NextPosition
		if err := position.Validate(); err != nil {
			return nil, err
		}
		if math.Abs(float64(position.Longitude-origin.Longitude)) > 180 {
			return nil, fmt.Errorf("routing: contour traversant l’antiméridien non pris en charge")
		}
		points = append(points, position)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Longitude < points[j].Longitude || points[i].Longitude == points[j].Longitude && points[i].Latitude < points[j].Latitude
	})
	unique := points[:0]
	for _, p := range points {
		if len(unique) == 0 || unique[len(unique)-1] != p {
			unique = append(unique, p)
		}
	}
	if len(unique) < 3 {
		return nil, fmt.Errorf("routing: front insuffisant pour un contour")
	}
	cross := func(a, b, c navigation.Position) float64 {
		return float64(b.Longitude-a.Longitude)*float64(c.Latitude-a.Latitude) - float64(b.Latitude-a.Latitude)*float64(c.Longitude-a.Longitude)
	}
	appendPoint := func(chain []navigation.Position, p navigation.Position) []navigation.Position {
		for len(chain) >= 2 && cross(chain[len(chain)-2], chain[len(chain)-1], p) <= 0 {
			chain = chain[:len(chain)-1]
		}
		return append(chain, p)
	}
	var lower, upper []navigation.Position
	for _, p := range unique {
		lower = appendPoint(lower, p)
	}
	for i := len(unique) - 1; i >= 0; i-- {
		upper = appendPoint(upper, unique[i])
	}
	contour := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	if len(contour) < 3 {
		return nil, fmt.Errorf("routing: front colinéaire sans surface")
	}
	return append(contour, contour[0]), nil
}

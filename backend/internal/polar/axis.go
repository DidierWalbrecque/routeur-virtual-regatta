package polar

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// axisBounds contient les indices qui encadrent une valeur sur un axe.
// Les deux indices sont identiques lorsque la valeur appartient exactement à l'axe.
type axisBounds struct {
	LowerIndex int
	UpperIndex int
}

// findAxisBounds recherche les deux positions qui encadrent value.
// L'axe doit avoir été validé comme strictement croissant.
func findAxisBounds[T ~float64](axis []T, value T) (axisBounds, error) {
	if len(axis) == 0 {
		return axisBounds{}, errors.New("polar: impossible de rechercher une valeur sur un axe vide")
	}

	numericValue := float64(value)
	if math.IsNaN(numericValue) || math.IsInf(numericValue, 0) {
		return axisBounds{}, errors.New("polar: la valeur recherchée sur l'axe doit être finie")
	}

	first := axis[0]
	last := axis[len(axis)-1]
	if value < first || value > last {
		return axisBounds{}, fmt.Errorf(
			"polar: valeur %g hors des limites de l'axe [%g, %g]",
			numericValue,
			float64(first),
			float64(last),
		)
	}

	upperIndex := sort.Search(len(axis), func(index int) bool {
		return axis[index] >= value
	})
	if axis[upperIndex] == value {
		return axisBounds{LowerIndex: upperIndex, UpperIndex: upperIndex}, nil
	}

	return axisBounds{LowerIndex: upperIndex - 1, UpperIndex: upperIndex}, nil
}

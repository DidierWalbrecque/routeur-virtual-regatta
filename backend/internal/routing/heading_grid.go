package routing

import (
	"errors"
	"fmt"
	"math"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

const (
	fullCircleDegrees = 360.0
	maxHeadingCount   = 3600
)

// GenerateHeadingGrid returns a regular full-circle grid starting at north.
// The spacing must divide 360 degrees exactly. The result includes 0 degrees
// and excludes the equivalent 360-degree heading.
func GenerateHeadingGrid(
	spacingDegrees float64,
) ([]navigation.Heading, error) {
	if math.IsNaN(spacingDegrees) ||
		math.IsInf(spacingDegrees, 0) ||
		spacingDegrees <= 0 ||
		spacingDegrees > fullCircleDegrees {
		return nil, errors.New(
			"routing: le pas angulaire doit être un nombre fini strictement positif et inférieur ou égal à 360 degrés",
		)
	}

	countFloat := fullCircleDegrees / spacingDegrees
	count := int(math.Round(countFloat))
	if count < 1 ||
		math.Abs(countFloat-float64(count)) > 1e-9 {
		return nil, fmt.Errorf(
			"routing: le pas angulaire %.12g° doit diviser exactement 360°",
			spacingDegrees,
		)
	}
	if count > maxHeadingCount {
		return nil, fmt.Errorf(
			"routing: le pas angulaire %.12g° produit %d caps, maximum autorisé %d",
			spacingDegrees,
			count,
			maxHeadingCount,
		)
	}

	headings := make([]navigation.Heading, count)
	for index := range headings {
		heading := navigation.Heading(float64(index) * spacingDegrees)
		if err := heading.Validate(); err != nil {
			return nil, fmt.Errorf(
				"routing: génération du cap %d impossible: %w",
				index+1,
				err,
			)
		}
		headings[index] = heading
	}
	return headings, nil
}

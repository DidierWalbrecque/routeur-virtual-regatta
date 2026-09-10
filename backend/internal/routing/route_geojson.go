package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

type geoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

type geoJSONFeature struct {
	Type       string            `json:"type"`
	Properties any               `json:"properties"`
	Geometry   geoJSONGeometry   `json:"geometry"`
}

type geoJSONGeometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

type routeGeoJSONProperties struct {
	Kind       string `json:"kind"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
}

type pointGeoJSONProperties struct {
	Kind string `json:"kind"`
	At   string `json:"at,omitempty"`
}

type stepGeoJSONProperties struct {
	Kind                string  `json:"kind"`
	Index               int     `json:"index"`
	At                  string  `json:"at"`
	Heading             float64 `json:"heading"`
	Sail                string  `json:"sail"`
	SpeedKnots          float64 `json:"speedKnots"`
	WindSpeedKnots      float64 `json:"windSpeedKnots"`
	WindDirection       float64 `json:"windDirection"`
	TrueWindAngle       float64 `json:"trueWindAngle"`
	Tack                string  `json:"tack"`
	RemainingDistanceNM float64 `json:"remainingDistanceNM"`
	ProgressNM          float64 `json:"progressNM"`
}

// MarshalGeoJSON serializes the reconstructed route as a GeoJSON feature
// collection suitable for a map. Coordinates follow the GeoJSON longitude,
// latitude order.
func (route IsochroneRoute) MarshalGeoJSON() ([]byte, error) {
	if err := validateRouteForGeoJSON(route); err != nil {
		return nil, err
	}

	positions := route.Positions()
	lineCoordinates := make([][]float64, len(positions))
	for index, position := range positions {
		lineCoordinates[index] = geoJSONCoordinates(position)
	}

	features := make([]geoJSONFeature, 0, len(route.Steps)+3)
	features = append(features, geoJSONFeature{
		Type: "Feature",
		Properties: routeGeoJSONProperties{
			Kind:       "route",
			StartedAt:  route.StartedAt.Format(time.RFC3339),
			FinishedAt: route.FinishedAt.Format(time.RFC3339),
		},
		Geometry: geoJSONGeometry{
			Type:        "LineString",
			Coordinates: lineCoordinates,
		},
	})
	features = append(features, geoJSONFeature{
		Type: "Feature",
		Properties: pointGeoJSONProperties{
			Kind: "start",
			At:   route.StartedAt.Format(time.RFC3339),
		},
		Geometry: geoJSONGeometry{
			Type:        "Point",
			Coordinates: geoJSONCoordinates(route.Origin),
		},
	})
	features = append(features, geoJSONFeature{
		Type: "Feature",
		Properties: pointGeoJSONProperties{
			Kind: "destination",
		},
		Geometry: geoJSONGeometry{
			Type:        "Point",
			Coordinates: geoJSONCoordinates(route.Destination),
		},
	})

	for index, routeStep := range route.Steps {
		step := routeStep.Node.Candidate.Step
		features = append(features, geoJSONFeature{
			Type: "Feature",
			Properties: stepGeoJSONProperties{
				Kind:                "step",
				Index:               index + 1,
				At:                  step.NextInstant.Format(time.RFC3339),
				Heading:             float64(step.Heading),
				Sail:                step.BestSail.SailName,
				SpeedKnots:          float64(step.BestSail.Speed),
				WindSpeedKnots:      float64(step.WindKnots),
				WindDirection:       float64(step.Wind.Direction),
				TrueWindAngle:       float64(step.WindAngle),
				Tack:                string(step.Tack),
				RemainingDistanceNM: float64(routeStep.Node.RemainingDistance),
				ProgressNM:          float64(routeStep.Node.Progress),
			},
			Geometry: geoJSONGeometry{
				Type:        "Point",
				Coordinates: geoJSONCoordinates(step.NextPosition),
			},
		})
	}

	data, err := json.MarshalIndent(geoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf(
			"routing: sérialisation GeoJSON impossible: %w",
			err,
		)
	}
	return data, nil
}

func validateRouteForGeoJSON(route IsochroneRoute) error {
	if err := route.Origin.Validate(); err != nil {
		return fmt.Errorf(
			"routing: export GeoJSON impossible: origine invalide: %w",
			err,
		)
	}
	if err := route.Destination.Validate(); err != nil {
		return fmt.Errorf(
			"routing: export GeoJSON impossible: destination invalide: %w",
			err,
		)
	}
	if route.StartedAt.IsZero() {
		return errors.New(
			"routing: export GeoJSON impossible: instant initial requis",
		)
	}
	if route.FinishedAt.IsZero() {
		return errors.New(
			"routing: export GeoJSON impossible: instant final requis",
		)
	}
	if len(route.Steps) == 0 {
		return errors.New(
			"routing: export GeoJSON impossible: route vide",
		)
	}

	expectedPosition := route.Origin
	expectedInstant := route.StartedAt
	for index, routeStep := range route.Steps {
		step := routeStep.Node.Candidate.Step
		if step.Position != expectedPosition {
			return fmt.Errorf(
				"routing: export GeoJSON impossible: position initiale du pas %d incohérente",
				index+1,
			)
		}
		if !step.Instant.Equal(expectedInstant) {
			return fmt.Errorf(
				"routing: export GeoJSON impossible: instant initial du pas %d incohérent",
				index+1,
			)
		}
		if err := step.NextPosition.Validate(); err != nil {
			return fmt.Errorf(
				"routing: export GeoJSON impossible: position finale du pas %d invalide: %w",
				index+1,
				err,
			)
		}
		if step.NextInstant.IsZero() ||
			!step.NextInstant.After(step.Instant) {
			return fmt.Errorf(
				"routing: export GeoJSON impossible: instant final du pas %d incohérent",
				index+1,
			)
		}
		expectedPosition = step.NextPosition
		expectedInstant = step.NextInstant
	}
	if !route.FinishedAt.Equal(expectedInstant) {
		return errors.New(
			"routing: export GeoJSON impossible: instant final de la route incohérent",
		)
	}
	return nil
}

func geoJSONCoordinates(position navigation.Position) []float64 {
	return []float64{
		float64(position.Longitude),
		float64(position.Latitude),
	}
}

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

const (
	maxExclusionZones      = 50
	maxExclusionZonePoints = 500
)

// exclusionZoneRequest est un polygone tracé par l'utilisateur sur la carte,
// que la route ne doit jamais traverser.
type exclusionZoneRequest struct {
	Name   string            `json:"name"`
	Points []positionRequest `json:"points"`
}

type zoneFeatureCollection struct {
	Type     string        `json:"type"`
	Features []zoneFeature `json:"features"`
}

type zoneFeature struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
	Geometry   zoneGeometry      `json:"geometry"`
}

type zoneGeometry struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// exclusionConstraintFromZones transforme les zones de la requête en contrainte
// de trajet. Elle renvoie nil lorsqu'aucune zone n'est fournie.
func exclusionConstraintFromZones(zones []exclusionZoneRequest) (*land.GeoJSONConstraint, error) {
	if len(zones) == 0 {
		return nil, nil
	}
	if len(zones) > maxExclusionZones {
		return nil, fmt.Errorf("au plus %d zones interdites sont acceptées, %d reçues", maxExclusionZones, len(zones))
	}

	collection := zoneFeatureCollection{Type: "FeatureCollection"}
	for index, zone := range zones {
		label := fmt.Sprintf("zone interdite %d", index+1)
		if zone.Name != "" {
			label = fmt.Sprintf("zone interdite %d (%s)", index+1, zone.Name)
		}
		if len(zone.Points) < 3 {
			return nil, fmt.Errorf("%s : au moins trois points sont requis", label)
		}
		if len(zone.Points) > maxExclusionZonePoints {
			return nil, fmt.Errorf("%s : au plus %d points sont acceptés", label, maxExclusionZonePoints)
		}

		ring := make([][2]float64, 0, len(zone.Points)+1)
		distinct := map[[2]float64]struct{}{}
		for pointIndex, point := range zone.Points {
			if math.IsNaN(point.Latitude) || math.IsInf(point.Latitude, 0) || point.Latitude < -90 || point.Latitude > 90 ||
				math.IsNaN(point.Longitude) || math.IsInf(point.Longitude, 0) || point.Longitude < -180 || point.Longitude > 180 {
				return nil, fmt.Errorf("%s : le point %d est hors limites", label, pointIndex+1)
			}
			coordinate := [2]float64{point.Longitude, point.Latitude}
			ring = append(ring, coordinate)
			distinct[coordinate] = struct{}{}
		}
		if len(distinct) < 3 {
			return nil, fmt.Errorf("%s : au moins trois points distincts sont requis", label)
		}
		if ring[0] != ring[len(ring)-1] {
			ring = append(ring, ring[0])
		}
		collection.Features = append(collection.Features, zoneFeature{
			Type:       "Feature",
			Properties: map[string]string{"name": zone.Name},
			Geometry:   zoneGeometry{Type: "Polygon", Coordinates: [][][2]float64{ring}},
		})
	}

	content, err := json.Marshal(collection)
	if err != nil {
		return nil, fmt.Errorf("zones interdites invalides : %w", err)
	}
	constraint, err := land.DecodeGeoJSON(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("zones interdites invalides : %w", err)
	}
	return constraint, nil
}

// allConstraints n'autorise un segment que si chaque contrainte l'autorise.
func allConstraints(constraints ...routing.Constraint) routing.Constraint {
	return routing.ConstraintFunc(func(ctx context.Context, segment routing.RouteSegment) (bool, error) {
		for _, constraint := range constraints {
			if constraint == nil {
				continue
			}
			allowed, err := constraint.Allows(ctx, segment)
			if err != nil || !allowed {
				return false, err
			}
		}
		return true, nil
	})
}

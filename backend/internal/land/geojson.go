// Package land loads terrestrial polygons used by routing constraints.
package land

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

type point struct {
	longitude float64
	latitude  float64
}

type polygon struct {
	rings              [][]point
	unwrappedRings     [][]point
	minLongitude       float64
	maxLongitude       float64
	minLatitude        float64
	maxLatitude        float64
	referenceLongitude float64
	boundsReady        bool
	// grid indexes the edges of large polygons; nil for small ones.
	grid *edgeGrid
}

// GeoJSONConstraint rejects route segments touching a terrestrial polygon.
type GeoJSONConstraint struct {
	polygons      []polygon
	index         polygonSpatialIndex
	coastalMargin navigation.NauticalMiles
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Geometry *geometry `json:"geometry"`
}

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// LoadGeoJSONFile loads Polygon and MultiPolygon features from a local file.
func LoadGeoJSONFile(path string) (*GeoJSONConstraint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf(
			"land: ouverture du GeoJSON %q impossible: %w",
			path,
			err,
		)
	}
	defer file.Close()

	constraint, err := DecodeGeoJSON(file)
	if err != nil {
		return nil, fmt.Errorf(
			"land: chargement du GeoJSON %q impossible: %w",
			path,
			err,
		)
	}
	return constraint, nil
}

// DecodeGeoJSON decodes a GeoJSON FeatureCollection containing terrestrial
// Polygon or MultiPolygon geometries.
func DecodeGeoJSON(reader io.Reader) (*GeoJSONConstraint, error) {
	if reader == nil {
		return nil, errors.New("land: lecteur GeoJSON requis")
	}

	decoder := json.NewDecoder(reader)
	var collection featureCollection
	if err := decoder.Decode(&collection); err != nil {
		return nil, fmt.Errorf("land: décodage GeoJSON impossible: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New(
				"land: contenu inattendu après le document GeoJSON",
			)
		}
		return nil, fmt.Errorf(
			"land: vérification de la fin du GeoJSON impossible: %w",
			err,
		)
	}
	if collection.Type != "FeatureCollection" {
		return nil, fmt.Errorf(
			"land: type GeoJSON %q invalide, FeatureCollection attendu",
			collection.Type,
		)
	}
	if len(collection.Features) == 0 {
		return nil, errors.New("land: aucune entité terrestre dans le GeoJSON")
	}

	polygons := make([]polygon, 0, len(collection.Features))
	for featureIndex, feature := range collection.Features {
		if feature.Geometry == nil {
			return nil, fmt.Errorf(
				"land: géométrie absente pour l'entité %d",
				featureIndex+1,
			)
		}
		decoded, err := decodeGeometry(*feature.Geometry)
		if err != nil {
			return nil, fmt.Errorf(
				"land: géométrie de l'entité %d invalide: %w",
				featureIndex+1,
				err,
			)
		}
		polygons = append(polygons, decoded...)
	}
	if len(polygons) == 0 {
		return nil, errors.New("land: aucun polygone terrestre décodé")
	}
	return &GeoJSONConstraint{
		polygons: polygons,
		index:    newPolygonSpatialIndex(polygons),
	}, nil
}

// WithCoastalMargin returns an independent constraint using the requested
// minimum distance from every coastline.
func (constraint *GeoJSONConstraint) WithCoastalMargin(
	margin navigation.NauticalMiles,
) (*GeoJSONConstraint, error) {
	if constraint == nil {
		return nil, errors.New("land: contrainte GeoJSON requise")
	}
	value := float64(margin)
	if math.IsNaN(value) || math.IsInf(value, 0) || margin < 0 {
		return nil, fmt.Errorf(
			"land: marge côtière %.12f NM invalide",
			value,
		)
	}
	result := *constraint
	result.coastalMargin = margin
	return &result, nil
}

// CoastalMargin returns the configured minimum distance from coastlines.
func (constraint *GeoJSONConstraint) CoastalMargin() navigation.NauticalMiles {
	if constraint == nil {
		return 0
	}
	return constraint.coastalMargin
}

// PolygonCount returns the number of decoded terrestrial polygons.
func (constraint *GeoJSONConstraint) PolygonCount() int {
	if constraint == nil {
		return 0
	}
	return len(constraint.polygons)
}

// Contains reports whether a position is on a terrestrial polygon or its
// boundary. It is used to reject invalid route endpoints before calculation.
func (constraint *GeoJSONConstraint) Contains(position navigation.Position) (bool, error) {
	if constraint == nil {
		return false, errors.New("land: contrainte GeoJSON requise")
	}
	if err := position.Validate(); err != nil {
		return false, fmt.Errorf("land: position invalide: %w", err)
	}
	value := point{longitude: float64(position.Longitude), latitude: float64(position.Latitude)}
	for _, polygon := range constraint.polygons {
		rings := make([][]point, len(polygon.rings))
		for index, ring := range polygon.rings {
			rings[index] = unwrapRing(ring, value.longitude)
		}
		if pointInPolygon(value, rings) {
			return true, nil
		}
	}
	return false, nil
}

func decodeGeometry(value geometry) ([]polygon, error) {
	switch value.Type {
	case "Polygon":
		var coordinates [][][]float64
		if err := json.Unmarshal(value.Coordinates, &coordinates); err != nil {
			return nil, fmt.Errorf(
				"coordonnées Polygon impossibles à décoder: %w",
				err,
			)
		}
		decoded, err := decodePolygon(coordinates)
		if err != nil {
			return nil, err
		}
		return []polygon{decoded}, nil
	case "MultiPolygon":
		var coordinates [][][][]float64
		if err := json.Unmarshal(value.Coordinates, &coordinates); err != nil {
			return nil, fmt.Errorf(
				"coordonnées MultiPolygon impossibles à décoder: %w",
				err,
			)
		}
		if len(coordinates) == 0 {
			return nil, errors.New("MultiPolygon vide")
		}
		result := make([]polygon, 0, len(coordinates))
		for index, polygonCoordinates := range coordinates {
			decoded, err := decodePolygon(polygonCoordinates)
			if err != nil {
				return nil, fmt.Errorf(
					"polygone %d invalide: %w",
					index+1,
					err,
				)
			}
			result = append(result, decoded)
		}
		return result, nil
	default:
		return nil, fmt.Errorf(
			"type de géométrie %q non pris en charge",
			value.Type,
		)
	}
}

func decodePolygon(coordinates [][][]float64) (polygon, error) {
	if len(coordinates) == 0 {
		return polygon{}, errors.New("Polygon sans anneau extérieur")
	}
	rings := make([][]point, len(coordinates))
	for ringIndex, coordinateRing := range coordinates {
		ring, err := decodeRing(coordinateRing)
		if err != nil {
			return polygon{}, fmt.Errorf(
				"anneau %d invalide: %w",
				ringIndex+1,
				err,
			)
		}
		rings[ringIndex] = ring
	}
	return polygon{rings: rings}, nil
}

func decodeRing(coordinates [][]float64) ([]point, error) {
	if len(coordinates) < 4 {
		return nil, errors.New("au moins quatre positions sont requises")
	}
	ring := make([]point, len(coordinates))
	for index, coordinate := range coordinates {
		if len(coordinate) < 2 {
			return nil, fmt.Errorf(
				"position %d sans longitude et latitude",
				index+1,
			)
		}
		longitude := coordinate[0]
		latitude := coordinate[1]
		if math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
			longitude < -180 || longitude > 180 {
			return nil, fmt.Errorf(
				"longitude %.12f invalide à la position %d",
				longitude,
				index+1,
			)
		}
		if math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
			latitude < -90 || latitude > 90 {
			return nil, fmt.Errorf(
				"latitude %.12f invalide à la position %d",
				latitude,
				index+1,
			)
		}
		ring[index] = point{
			longitude: longitude,
			latitude:  latitude,
		}
	}
	first := ring[0]
	last := ring[len(ring)-1]
	if first != last {
		return nil, errors.New(
			"la première et la dernière position doivent être identiques",
		)
	}
	return ring, nil
}

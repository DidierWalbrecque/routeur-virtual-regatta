package weather

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"time"

	gribfile "github.com/skysparq/grib2-go/file"
	"github.com/skysparq/grib2-go/record"
	"github.com/skysparq/grib2-go/templates"
)

const (
	meteorologicalDiscipline = 0
	momentumCategory         = 2
	uComponentParameter      = 2
	vComponentParameter      = 3
	heightAboveGroundSurface = 103
	tenMeters                = 10
)

type windComponent uint8

const (
	windComponentUnknown windComponent = iota
	windComponentU
	windComponentV
)

// GRIB2WindGrid contient les composantes U et V à 10 mètres d'une échéance
// NOAA, alignées sur une même grille régulière.
//
// Les composantes sont exprimées en mètres par seconde. Les tableaux sont
// rangés ligne par ligne, du nord-ouest vers le sud-est.
type GRIB2WindGrid struct {
	ValidAt   time.Time
	Width     int
	Height    int
	Latitudes []float64
	Longitudes []float64
	U         []float64
	V         []float64
}

// Validate vérifie que les coordonnées et les composantes décrivent toutes la
// même grille et ne contiennent que des valeurs finies.
func (grid GRIB2WindGrid) Validate() error {
	if grid.ValidAt.IsZero() {
		return fmt.Errorf("weather: l'échéance de la grille GRIB2 ne doit pas être nulle")
	}
	if grid.Width <= 0 || grid.Height <= 0 {
		return fmt.Errorf(
			"weather: dimensions de grille GRIB2 invalides: %dx%d",
			grid.Width,
			grid.Height,
		)
	}
	maxInt := int(^uint(0) >> 1)
	if grid.Width > maxInt/grid.Height {
		return fmt.Errorf("weather: dimensions de grille GRIB2 trop grandes")
	}

	expected := grid.Width * grid.Height
	lengths := []struct {
		name   string
		length int
	}{
		{name: "latitudes", length: len(grid.Latitudes)},
		{name: "longitudes", length: len(grid.Longitudes)},
		{name: "composante U", length: len(grid.U)},
		{name: "composante V", length: len(grid.V)},
	}
	for _, item := range lengths {
		if item.length != expected {
			return fmt.Errorf(
				"weather: %s de longueur %d, attendu %d pour une grille %dx%d",
				item.name,
				item.length,
				expected,
				grid.Width,
				grid.Height,
			)
		}
	}

	for index := range expected {
		if !finite(grid.Latitudes[index]) ||
			grid.Latitudes[index] < -90 ||
			grid.Latitudes[index] > 90 {
			return fmt.Errorf(
				"weather: latitude invalide au point %d: %v",
				index,
				grid.Latitudes[index],
			)
		}
		if !finite(grid.Longitudes[index]) ||
			grid.Longitudes[index] < -180 ||
			grid.Longitudes[index] >= 360 {
			return fmt.Errorf(
				"weather: longitude invalide au point %d: %v",
				index,
				grid.Longitudes[index],
			)
		}
		if !finite(grid.U[index]) {
			return fmt.Errorf(
				"weather: composante U invalide au point %d: %v",
				index,
				grid.U[index],
			)
		}
		if !finite(grid.V[index]) {
			return fmt.Errorf(
				"weather: composante V invalide au point %d: %v",
				index,
				grid.V[index],
			)
		}
	}

	return nil
}

// LoadGRIB2WindGrid charge dans un fichier GRIB2 les champs UGRD et VGRD à
// 10 mètres. Les deux champs doivent partager la même grille et la même
// échéance.
func LoadGRIB2WindGrid(path string) (GRIB2WindGrid, error) {
	source, err := os.Open(path)
	if err != nil {
		return GRIB2WindGrid{}, fmt.Errorf(
			"weather: ouverture du fichier GRIB2 %q impossible: %w",
			path,
			err,
		)
	}

	gribFile := gribfile.NewGribFile(source, templates.Version33())
	defer gribFile.Close()

	var grid GRIB2WindGrid
	var foundU, foundV bool
	var gridTemplateNumber int
	var gridTemplateData, optionalPointListData []byte

	for indexed, parseErr := range gribFile.Records {
		if parseErr != nil {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: lecture du message GRIB2 %d impossible: %w",
				indexed.MessageNumber,
				parseErr,
			)
		}

		gribRecord := indexed.Record
		productDefinition, err := gribRecord.Product.Definition()
		if err != nil {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: lecture du produit du message GRIB2 %d impossible: %w",
				indexed.MessageNumber,
				err,
			)
		}
		header := productDefinition.Header(gribRecord.Identification)
		component := classifyTenMeterWind(
			gribRecord.Indicator.Discipline,
			header.ParameterCategory,
			header.ParameterNumber,
			header.FirstSurfaceType,
			header.FirstSurfaceValue,
		)
		if component == windComponentUnknown {
			continue
		}
		if component == windComponentU && foundU {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: plusieurs composantes U à 10 mètres trouvées",
			)
		}
		if component == windComponentV && foundV {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: plusieurs composantes V à 10 mètres trouvées",
			)
		}

		if !foundU && !foundV {
			grid.ValidAt = header.Start
			gridTemplateNumber = gribRecord.Grid.GridDefinitionTemplateNumber
			gridTemplateData = append(
				[]byte(nil),
				gribRecord.Grid.GridDefinitionTemplateData...,
			)
			optionalPointListData = append(
				[]byte(nil),
				gribRecord.Grid.OptionalPointListData...,
			)

			gridDefinition, err := gribRecord.Grid.Definition()
			if err != nil {
				return GRIB2WindGrid{}, fmt.Errorf(
					"weather: lecture de la grille du message GRIB2 %d impossible: %w",
					indexed.MessageNumber,
					err,
				)
			}
			points, err := gridDefinition.Points()
			if err != nil {
				return GRIB2WindGrid{}, fmt.Errorf(
					"weather: calcul des coordonnées du message GRIB2 %d impossible: %w",
					indexed.MessageNumber,
					err,
				)
			}
			grid.Width = gridDefinition.XVals()
			grid.Height = gridDefinition.YVals()
			grid.Latitudes = points.Lats
			grid.Longitudes = points.Lngs
		} else {
			if !header.Start.Equal(grid.ValidAt) {
				return GRIB2WindGrid{}, fmt.Errorf(
					"weather: échéances U/V différentes: %s et %s",
					grid.ValidAt.Format(time.RFC3339),
					header.Start.Format(time.RFC3339),
				)
			}
			if gribRecord.Grid.GridDefinitionTemplateNumber != gridTemplateNumber ||
				!bytes.Equal(
					gribRecord.Grid.GridDefinitionTemplateData,
					gridTemplateData,
				) ||
				!bytes.Equal(
					gribRecord.Grid.OptionalPointListData,
					optionalPointListData,
				) {
				return GRIB2WindGrid{}, fmt.Errorf(
					"weather: les composantes U et V n'utilisent pas la même grille",
				)
			}
		}

		dataDefinition, err := gribRecord.DataRepresentation.Definition()
		if err != nil {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: lecture de la représentation du message GRIB2 %d impossible: %w",
				indexed.MessageNumber,
				err,
			)
		}
		values, err := dataDefinition.GetValues(gribRecord)
		if err != nil {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: décodage des valeurs du message GRIB2 %d impossible: %w",
				indexed.MessageNumber,
				err,
			)
		}

		switch component {
		case windComponentU:
			grid.U = values
			foundU = true
		case windComponentV:
			grid.V = values
			foundV = true
		}
	}

	if !foundU || !foundV {
		return GRIB2WindGrid{}, fmt.Errorf(
			"weather: composantes du vent à 10 mètres absentes: U=%t, V=%t",
			foundU,
			foundV,
		)
	}
	if err := grid.Validate(); err != nil {
		return GRIB2WindGrid{}, fmt.Errorf(
			"weather: grille de vent GRIB2 invalide: %w",
			err,
		)
	}

	return grid, nil
}

func classifyTenMeterWind(
	discipline,
	category,
	parameter,
	surfaceType int,
	surfaceValue float64,
) windComponent {
	if discipline != meteorologicalDiscipline ||
		category != momentumCategory ||
		surfaceType != heightAboveGroundSurface ||
		surfaceValue != tenMeters {
		return windComponentUnknown
	}

	switch parameter {
	case uComponentParameter:
		return windComponentU
	case vComponentParameter:
		return windComponentV
	default:
		return windComponentUnknown
	}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

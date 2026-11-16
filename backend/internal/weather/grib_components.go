package weather

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"time"

	gribfile "github.com/skysparq/grib2-go/file"
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
// Les composantes sont exprimées en mètres par seconde. U et V sont rangés
// ligne par ligne, du nord-ouest vers le sud-est. La grille étant régulière,
// seuls ses axes sont conservés : Latitudes contient une valeur par ligne
// (Height valeurs) et Longitudes une valeur par colonne (Width valeurs), ce qui
// évite de garder deux coordonnées par point en mémoire. U et V sont stockés
// en float32 : leur précision (environ 7 chiffres significatifs) dépasse
// largement celle des valeurs GFS, codées au dixième de m/s près, et divise
// par deux la mémoire occupée par chaque échéance.
type GRIB2WindGrid struct {
	ValidAt    time.Time
	Width      int
	Height     int
	Latitudes  []float64
	Longitudes []float64
	U          []float32
	V          []float32
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
	if len(grid.Latitudes) != grid.Height {
		return fmt.Errorf(
			"weather: axe des latitudes de longueur %d, attendu %d (une par ligne)",
			len(grid.Latitudes),
			grid.Height,
		)
	}
	if len(grid.Longitudes) != grid.Width {
		return fmt.Errorf(
			"weather: axe des longitudes de longueur %d, attendu %d (une par colonne)",
			len(grid.Longitudes),
			grid.Width,
		)
	}
	lengths := []struct {
		name   string
		length int
	}{
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

	for row, latitude := range grid.Latitudes {
		if !finite(latitude) || latitude < -90 || latitude > 90 {
			return fmt.Errorf(
				"weather: latitude invalide à la ligne %d: %v",
				row,
				latitude,
			)
		}
	}
	for column, longitude := range grid.Longitudes {
		if !finite(longitude) || longitude < -180 || longitude >= 360 {
			return fmt.Errorf(
				"weather: longitude invalide à la colonne %d: %v",
				column,
				longitude,
			)
		}
	}

	for index := range expected {
		if !finite(float64(grid.U[index])) {
			return fmt.Errorf(
				"weather: composante U invalide au point %d: %v",
				index,
				grid.U[index],
			)
		}
		if !finite(float64(grid.V[index])) {
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
			latitudes, longitudes, err := regularGridAxes(
				points.Lats,
				points.Lngs,
				grid.Width,
				grid.Height,
			)
			if err != nil {
				return GRIB2WindGrid{}, fmt.Errorf(
					"weather: grille du message GRIB2 %d non régulière: %w",
					indexed.MessageNumber,
					err,
				)
			}
			grid.Latitudes = latitudes
			grid.Longitudes = longitudes
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

		compact, err := windComponentToFloat32(values)
		if err != nil {
			return GRIB2WindGrid{}, fmt.Errorf(
				"weather: valeurs du message GRIB2 %d invalides: %w",
				indexed.MessageNumber,
				err,
			)
		}

		switch component {
		case windComponentU:
			grid.U = compact
			foundU = true
		case windComponentV:
			grid.V = compact
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

// windComponentToFloat32 convertit une composante de vent décodée en float64
// vers le stockage compact en float32. Une valeur non finie ou trop grande
// pour un float32 est refusée plutôt que transformée en infini.
func windComponentToFloat32(values []float64) ([]float32, error) {
	compact := make([]float32, len(values))
	for index, value := range values {
		if !finite(value) || math.Abs(value) > math.MaxFloat32 {
			return nil, fmt.Errorf(
				"composante non représentable au point %d: %v",
				index,
				value,
			)
		}
		compact[index] = float32(value)
	}
	return compact, nil
}

// regularGridAxesTolerance borne l'écart admis, en degrés, entre la
// coordonnée d'un point et celle de l'axe de sa ligne ou de sa colonne.
const regularGridAxesTolerance = 1e-6

// regularGridAxes réduit les coordonnées point par point d'une grille régulière
// à ses deux axes : la latitude de chaque ligne et la longitude de chaque
// colonne. Elle échoue si un point s'écarte de l'axe de sa ligne ou de sa
// colonne, car l'interpolation suppose une grille régulière.
func regularGridAxes[T float32 | float64](
	pointLatitudes, pointLongitudes []T,
	width, height int,
) ([]float64, []float64, error) {
	if width <= 0 || height <= 0 {
		return nil, nil, fmt.Errorf("dimensions invalides: %dx%d", width, height)
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height {
		return nil, nil, fmt.Errorf("dimensions trop grandes")
	}
	expected := width * height
	if len(pointLatitudes) != expected || len(pointLongitudes) != expected {
		return nil, nil, fmt.Errorf(
			"%d latitudes et %d longitudes, attendu %d pour une grille %dx%d",
			len(pointLatitudes),
			len(pointLongitudes),
			expected,
			width,
			height,
		)
	}

	latitudes := make([]float64, height)
	for row := range height {
		latitudes[row] = float64(pointLatitudes[row*width])
	}
	longitudes := make([]float64, width)
	for column := range width {
		longitudes[column] = float64(pointLongitudes[column])
	}

	for row := range height {
		for column := range width {
			index := row*width + column
			if math.Abs(float64(pointLatitudes[index])-latitudes[row]) > regularGridAxesTolerance {
				return nil, nil, fmt.Errorf(
					"latitude %v au point %d, attendu %v pour la ligne %d",
					pointLatitudes[index],
					index,
					latitudes[row],
					row,
				)
			}
			if math.Abs(float64(pointLongitudes[index])-longitudes[column]) > regularGridAxesTolerance {
				return nil, nil, fmt.Errorf(
					"longitude %v au point %d, attendu %v pour la colonne %d",
					pointLongitudes[index],
					index,
					longitudes[column],
					column,
				)
			}
		}
	}

	return latitudes, longitudes, nil
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

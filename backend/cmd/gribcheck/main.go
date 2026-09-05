// Command gribcheck diagnoses whether one or more GRIB2 files contain
// complete ten-meter wind grids and can answer a weather query.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type filePaths []string

func (paths *filePaths) Set(value string) error {
	if value == "" {
		return errors.New("le chemin du fichier GRIB2 ne doit pas être vide")
	}
	*paths = append(*paths, value)
	return nil
}

func (paths *filePaths) String() string {
	return strings.Join(*paths, ",")
}

type options struct {
	files       filePaths
	latitude    float64
	longitude   float64
	hasPosition bool
	instant     time.Time
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		printErrorChain(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}

	grids := make([]weather.GRIB2WindGrid, 0, len(options.files))
	for index, filePath := range options.files {
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf(
				"lecture des informations de %q impossible: %w",
				filePath,
				err,
			)
		}

		if len(options.files) == 1 {
			fmt.Fprintf(stdout, "Fichier : %s\n", filePath)
		} else {
			fmt.Fprintf(stdout, "Fichier %d : %s\n", index+1, filePath)
		}
		fmt.Fprintf(stdout, "Taille : %d octets\n", info.Size())
		fmt.Fprintln(stdout, "Extraction des composantes U et V à 10 mètres...")

		grid, err := weather.LoadGRIB2WindGrid(filePath)
		if err != nil {
			return err
		}
		grids = append(grids, grid)

		minU, maxU := minMax(grid.U)
		minV, maxV := minMax(grid.V)

		fmt.Fprintf(stdout, "Échéance : %s\n", grid.ValidAt.Format(time.RFC3339))
		fmt.Fprintf(stdout, "Grille : %d x %d\n", grid.Width, grid.Height)
		fmt.Fprintf(stdout, "Points : %d\n", len(grid.U))
		fmt.Fprintf(stdout, "Composante U : %.6f à %.6f m/s\n", minU, maxU)
		fmt.Fprintf(stdout, "Composante V : %.6f à %.6f m/s\n", minV, maxV)
		fmt.Fprintln(stdout, "Composantes U et V à 10 mètres validées.")
	}

	if !options.hasPosition {
		return nil
	}

	queryAt := options.instant
	if queryAt.IsZero() {
		queryAt = grids[0].ValidAt
	}

	provider, err := weather.NewGRIB2WeatherProvider(grids...)
	if err != nil {
		return err
	}
	wind, err := provider.WindAt(
		context.Background(),
		navigation.Position{
			Latitude:  navigation.Latitude(options.latitude),
			Longitude: navigation.Longitude(options.longitude),
		},
		queryAt,
	)
	if err != nil {
		return err
	}
	speedInKnots, err := navigation.KnotsFromMetersPerSecond(wind.Speed)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		stdout,
		"Position : %.6f°, %.6f°\n",
		options.latitude,
		options.longitude,
	)
	fmt.Fprintf(stdout, "Instant demandé : %s\n", queryAt.Format(time.RFC3339))

	if len(grids) == 1 && queryAt.Equal(grids[0].ValidAt) {
		u, v, err := grids[0].ComponentsAt(navigation.Position{
			Latitude:  navigation.Latitude(options.latitude),
			Longitude: navigation.Longitude(options.longitude),
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "U interpolé : %.6f m/s\n", u)
		fmt.Fprintf(stdout, "V interpolé : %.6f m/s\n", v)
	}

	fmt.Fprintf(
		stdout,
		"Vent : %.6f m/s | %.6f nd | direction %.3f°\n",
		wind.Speed,
		speedInKnots,
		wind.Direction,
	)

	return nil
}

func parseOptions(args []string) (options, error) {
	result := options{
		latitude:  math.NaN(),
		longitude: math.NaN(),
	}
	flags := flag.NewFlagSet("gribcheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	flags.Var(
		&result.files,
		"file",
		"chemin d'un fichier GRIB2 ; option répétable",
	)
	flags.Float64Var(
		&result.latitude,
		"latitude",
		math.NaN(),
		"latitude à interroger en degrés décimaux",
	)
	flags.Float64Var(
		&result.longitude,
		"longitude",
		math.NaN(),
		"longitude à interroger en degrés décimaux",
	)
	atText := flags.String(
		"at",
		"",
		"instant à interroger au format RFC3339",
	)
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("arguments invalides: %w", err)
	}

	if len(result.files) == 0 {
		if flags.NArg() != 1 {
			return options{}, errors.New(
				"au moins un chemin GRIB2 est requis avec -file",
			)
		}
		result.files = append(result.files, flags.Arg(0))
	} else if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"arguments inattendus: %v",
			flags.Args(),
		)
	}

	hasLatitude := !math.IsNaN(result.latitude)
	hasLongitude := !math.IsNaN(result.longitude)
	if hasLatitude != hasLongitude {
		return options{}, errors.New(
			"-latitude et -longitude doivent être fournis ensemble",
		)
	}
	result.hasPosition = hasLatitude

	if *atText != "" {
		if !result.hasPosition {
			return options{}, errors.New(
				"-at nécessite également -latitude et -longitude",
			)
		}
		instant, err := time.Parse(time.RFC3339, *atText)
		if err != nil {
			return options{}, fmt.Errorf(
				"instant %q invalide, format RFC3339 attendu: %w",
				*atText,
				err,
			)
		}
		result.instant = instant
	}

	if len(result.files) > 1 &&
		result.hasPosition &&
		result.instant.IsZero() {
		return options{}, errors.New(
			"-at est requis pour interroger plusieurs fichiers GRIB2",
		)
	}

	return result, nil
}

func minMax(values []float64) (float64, float64) {
	minimum := math.Inf(1)
	maximum := math.Inf(-1)
	for _, value := range values {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return minimum, maximum
}

func printErrorChain(stderr io.Writer, err error) {
	for depth, cause := 0, err; cause != nil; depth, cause = depth+1, errors.Unwrap(cause) {
		fmt.Fprintf(stderr, "Cause %d (%T) : %v\n", depth, cause, cause)
	}
}

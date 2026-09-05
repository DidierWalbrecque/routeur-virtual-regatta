// Command gribcheck diagnoses whether a GRIB2 file contains a complete
// ten-meter wind grid.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		printErrorChain(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("gribcheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	filePath := flags.String("file", "", "chemin du fichier GRIB2")
	latitude := flags.Float64(
		"latitude",
		math.NaN(),
		"latitude à interroger en degrés décimaux",
	)
	longitude := flags.Float64(
		"longitude",
		math.NaN(),
		"longitude à interroger en degrés décimaux",
	)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("arguments invalides: %w", err)
	}

	if *filePath == "" && flags.NArg() == 1 {
		*filePath = flags.Arg(0)
	}
	if *filePath == "" {
		return errors.New("le chemin du fichier GRIB2 est requis avec -file")
	}
	if flags.NArg() > 0 && flags.Arg(0) != *filePath {
		return fmt.Errorf("arguments inattendus: %v", flags.Args())
	}

	hasLatitude := !math.IsNaN(*latitude)
	hasLongitude := !math.IsNaN(*longitude)
	if hasLatitude != hasLongitude {
		return errors.New(
			"-latitude et -longitude doivent être fournis ensemble",
		)
	}

	info, err := os.Stat(*filePath)
	if err != nil {
		return fmt.Errorf(
			"lecture des informations de %q impossible: %w",
			*filePath,
			err,
		)
	}

	fmt.Fprintf(stdout, "Fichier : %s\n", *filePath)
	fmt.Fprintf(stdout, "Taille : %d octets\n", info.Size())
	fmt.Fprintln(stdout, "Extraction des composantes U et V à 10 mètres...")

	grid, err := weather.LoadGRIB2WindGrid(*filePath)
	if err != nil {
		return err
	}

	minU, maxU := minMax(grid.U)
	minV, maxV := minMax(grid.V)

	fmt.Fprintf(stdout, "Échéance : %s\n", grid.ValidAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "Grille : %d x %d\n", grid.Width, grid.Height)
	fmt.Fprintf(stdout, "Points : %d\n", len(grid.U))
	fmt.Fprintf(stdout, "Composante U : %.6f à %.6f m/s\n", minU, maxU)
	fmt.Fprintf(stdout, "Composante V : %.6f à %.6f m/s\n", minV, maxV)
	fmt.Fprintln(stdout, "Composantes U et V à 10 mètres validées.")

	if hasLatitude {
		position := navigation.Position{
			Latitude:  navigation.Latitude(*latitude),
			Longitude: navigation.Longitude(*longitude),
		}
		u, v, err := grid.ComponentsAt(position)
		if err != nil {
			return err
		}
		wind, err := navigation.WindFromComponents(u, v)
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
			*latitude,
			*longitude,
		)
		fmt.Fprintf(stdout, "U interpolé : %.6f m/s\n", u)
		fmt.Fprintf(stdout, "V interpolé : %.6f m/s\n", v)
		fmt.Fprintf(
			stdout,
			"Vent : %.6f m/s | %.6f nd | direction %.3f°\n",
			wind.Speed,
			speedInKnots,
			wind.Direction,
		)
	}

	return nil
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

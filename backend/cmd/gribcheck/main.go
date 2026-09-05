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

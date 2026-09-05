// Command gribcheck diagnoses whether a GRIB2 file can be decoded by Squall.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	grib "github.com/mmp/squall"
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

	file, err := os.Open(*filePath)
	if err != nil {
		return fmt.Errorf("ouverture de %q impossible: %w", *filePath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("lecture des informations de %q impossible: %w", *filePath, err)
	}

	fmt.Fprintf(stdout, "Fichier : %s\n", *filePath)
	fmt.Fprintf(stdout, "Taille : %d octets\n", info.Size())
	fmt.Fprintln(stdout, "Décodage séquentiel strict (aucune erreur ignorée)...")

	fields, err := grib.ReadWithOptions(file, grib.WithSequential())
	if err != nil {
		return fmt.Errorf("décodage GRIB2 impossible: %w", err)
	}
	if len(fields) == 0 {
		return errors.New("aucun message GRIB2 décodé sans erreur explicite")
	}

	fmt.Fprintf(stdout, "Messages décodés : %d\n", len(fields))
	for index, field := range fields {
		fmt.Fprintf(
			stdout,
			"%d. %s | %s | grille %dx%d | %d points\n",
			index+1,
			field.Parameter.String(),
			field.Level,
			field.GridNi,
			field.GridNj,
			field.NumPoints,
		)
	}

	return nil
}

func printErrorChain(stderr io.Writer, err error) {
	for depth, cause := 0, err; cause != nil; depth, cause = depth+1, errors.Unwrap(cause) {
		fmt.Fprintf(stderr, "Cause %d (%T) : %v\n", depth, cause, cause)
	}
}

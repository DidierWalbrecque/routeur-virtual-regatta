// Command gribcheck diagnoses whether a GRIB2 file can be fully decoded.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/skysparq/grib2-go/file"
	"github.com/skysparq/grib2-go/templates"
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

	source, err := os.Open(*filePath)
	if err != nil {
		return fmt.Errorf("ouverture de %q impossible: %w", *filePath, err)
	}

	info, err := source.Stat()
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("lecture des informations de %q impossible: %w", *filePath, err)
	}

	fmt.Fprintf(stdout, "Fichier : %s\n", *filePath)
	fmt.Fprintf(stdout, "Taille : %d octets\n", info.Size())
	fmt.Fprintln(stdout, "Décodage complet des messages, coordonnées et valeurs...")

	gribFile := file.NewGribFile(source, templates.Version33())
	defer gribFile.Close()

	messageCount := 0
	for indexed, parseErr := range gribFile.Records {
		if parseErr != nil {
			return fmt.Errorf("lecture du message %d impossible: %w", indexed.MessageNumber, parseErr)
		}

		griddedValues, err := indexed.Record.GetGriddedValues()
		if err != nil {
			return fmt.Errorf(
				"décodage des valeurs du message %d impossible: %w",
				indexed.MessageNumber,
				err,
			)
		}

		fmt.Fprintf(
			stdout,
			"%d. grille 3.%d | produit 4.%d | données 5.%d | %d points décodés\n",
			indexed.MessageNumber,
			indexed.Record.Grid.GridDefinitionTemplateNumber,
			indexed.Record.Product.ProductDefinitionTemplateNumber,
			indexed.Record.DataRepresentation.DataRepresentationTemplateNumber,
			len(griddedValues.Values),
		)
		messageCount++
	}

	if messageCount == 0 {
		return errors.New("aucun message GRIB2 trouvé")
	}

	fmt.Fprintf(stdout, "Messages entièrement décodés : %d\n", messageCount)
	return nil
}

func printErrorChain(stderr io.Writer, err error) {
	for depth, cause := 0, err; cause != nil; depth, cause = depth+1, errors.Unwrap(cause) {
		fmt.Fprintf(stderr, "Cause %d (%T) : %v\n", depth, cause, cause)
	}
}

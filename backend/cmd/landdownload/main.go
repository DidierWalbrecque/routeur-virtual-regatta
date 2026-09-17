// Command landdownload downloads and validates the Natural Earth 1:10m
// terrestrial dataset used by the routing engine.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
)

type options struct {
	outputDirectory string
}

type naturalEarthDownloadClient interface {
	Download(context.Context, string) (land.NaturalEarthDownload, error)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	return runWithDownloader(
		args,
		stdout,
		land.NewNaturalEarthDownloader(),
	)
}

func runWithDownloader(
	args []string,
	stdout io.Writer,
	downloader naturalEarthDownloadClient,
) error {
	if stdout == nil {
		return errors.New("landdownload: sortie standard requise")
	}
	if downloader == nil {
		return errors.New("landdownload: téléchargeur Natural Earth requis")
	}

	opts, err := parseOptions(args)
	if err != nil {
		return err
	}

	fmt.Fprintln(
		stdout,
		"Téléchargement et validation de Natural Earth 1:10m...",
	)
	result, err := downloader.Download(
		context.Background(),
		opts.outputDirectory,
	)
	if err != nil {
		return fmt.Errorf(
			"landdownload: préparation de Natural Earth impossible: %w",
			err,
		)
	}

	action := "Téléchargé"
	if result.Reused {
		action = "Réutilisé"
	}
	fmt.Fprintf(
		stdout,
		"%s : %s | %d octets | %d polygones\n",
		action,
		result.Path,
		result.Size,
		result.PolygonCount,
	)
	return nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("landdownload", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts options
	flags.StringVar(
		&opts.outputDirectory,
		"output",
		"../data/land",
		"dossier de destination du GeoJSON Natural Earth",
	)
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf(
			"landdownload: arguments invalides: %w",
			err,
		)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"landdownload: argument inattendu %q",
			flags.Arg(0),
		)
	}
	opts.outputDirectory = strings.TrimSpace(opts.outputDirectory)
	if opts.outputDirectory == "" {
		return options{}, errors.New(
			"landdownload: le dossier de destination est requis",
		)
	}
	return opts, nil
}

// Command gribdownload downloads filtered NOAA GFS wind forecasts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type options struct {
	date            string
	cycle           int
	forecastHours   []int
	outputDirectory string
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}

	downloader := weather.NewNOAAGFSDownloader()
	for index, forecastHour := range options.forecastHours {
		request, err := weather.NewGFSForecastRequest(
			options.date,
			options.cycle,
			forecastHour,
		)
		if err != nil {
			return err
		}

		fmt.Fprintf(
			stdout,
			"Téléchargement de %s...\n",
			request.SourceFileName(),
		)
		path, err := downloader.Download(
			context.Background(),
			request,
			options.outputDirectory,
		)
		if err != nil {
			return fmt.Errorf(
				"téléchargement de f%03d impossible: %w",
				forecastHour,
				err,
			)
		}
		fmt.Fprintf(
			stdout,
			"Disponible : %s | échéance %s\n",
			path,
			request.ValidAt().Format("2006-01-02T15:04:05Z07:00"),
		)
		if index+1 < len(options.forecastHours) {
			time.Sleep(time.Second)
		}
	}

	return nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("gribdownload", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	date := flags.String(
		"date",
		"",
		"date du run GFS au format YYYYMMDD",
	)
	cycleText := flags.String(
		"cycle",
		"",
		"cycle GFS: 00, 06, 12 ou 18",
	)
	hoursText := flags.String(
		"hours",
		"0,3,6",
		"échéances séparées par des virgules",
	)
	outputDirectory := flags.String(
		"output",
		"../data/weather",
		"dossier de destination",
	)
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("arguments invalides: %w", err)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"arguments inattendus: %v",
			flags.Args(),
		)
	}
	if strings.TrimSpace(*date) == "" {
		return options{}, errors.New("-date est requis au format YYYYMMDD")
	}
	if strings.TrimSpace(*cycleText) == "" {
		return options{}, errors.New("-cycle est requis: 00, 06, 12 ou 18")
	}
	cycle, err := strconv.Atoi(*cycleText)
	if err != nil {
		return options{}, fmt.Errorf(
			"cycle %q invalide: %w",
			*cycleText,
			err,
		)
	}

	forecastHours, err := parseForecastHours(*hoursText)
	if err != nil {
		return options{}, err
	}
	for _, forecastHour := range forecastHours {
		if _, err := weather.NewGFSForecastRequest(
			*date,
			cycle,
			forecastHour,
		); err != nil {
			return options{}, err
		}
	}
	if strings.TrimSpace(*outputDirectory) == "" {
		return options{}, errors.New("-output ne doit pas être vide")
	}

	return options{
		date:            *date,
		cycle:           cycle,
		forecastHours:   forecastHours,
		outputDirectory: *outputDirectory,
	}, nil
}

func parseForecastHours(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	hours := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf(
				"liste d'échéances %q invalide",
				value,
			)
		}
		hour, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf(
				"échéance %q invalide: %w",
				part,
				err,
			)
		}
		if _, exists := seen[hour]; exists {
			return nil, fmt.Errorf(
				"échéance f%03d présente plusieurs fois",
				hour,
			)
		}
		seen[hour] = struct{}{}
		hours = append(hours, hour)
	}
	sort.Ints(hours)
	return hours, nil
}

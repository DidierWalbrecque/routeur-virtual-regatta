// Command gribrefresh downloads the most recent available NOAA GFS run.
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
	cycle           string
	forecastHours   []int
	outputDirectory string
	lookbackHours   int
}

type downloader interface {
	Download(context.Context, weather.GFSForecastRequest, string) (string, error)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, weather.NewNOAAGFSDownloader(), time.Now); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, client downloader, now func() time.Time) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("gribrefresh: téléchargeur NOAA requis")
	}

	for _, candidate := range candidateRuns(options, now().UTC()) {
		fmt.Fprintf(stdout, "Recherche du run GFS %s...\n", candidateLabel(candidate))
		paths, err := downloadRun(context.Background(), client, candidate, options, stdout)
		if err == nil {
			fmt.Fprintf(stdout, "Run disponible : %s | %d fichier(s)\n", candidateLabel(candidate), len(paths))
			return nil
		}
		fmt.Fprintf(stdout, "Run indisponible : %v\n", err)
	}

	return fmt.Errorf("gribrefresh: aucun run GFS disponible dans les %d dernières heures", options.lookbackHours)
}

func downloadRun(ctx context.Context, client downloader, candidate weather.GFSForecastRequest, options options, stdout io.Writer) ([]string, error) {
	paths := make([]string, 0, len(options.forecastHours))
	for index, forecastHour := range options.forecastHours {
		request := candidate
		request.ForecastHour = forecastHour
		path, err := client.Download(ctx, request, options.outputDirectory)
		if err != nil {
			return nil, fmt.Errorf("échéance f%03d impossible: %w", forecastHour, err)
		}
		paths = append(paths, path)
		fmt.Fprintf(stdout, "Disponible : %s | échéance %s\n", path, request.ValidAt().Format(time.RFC3339))
		if index+1 < len(options.forecastHours) {
			time.Sleep(time.Second)
		}
	}
	return paths, nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("gribrefresh", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	date := flags.String("date", "", "date du run GFS au format YYYYMMDD; avec -cycle pour forcer un run")
	cycle := flags.String("cycle", "", "cycle GFS 00, 06, 12 ou 18; avec -date pour forcer un run")
	hours := flags.String("hours", "0,3,6,9,12", "échéances séparées par des virgules")
	output := flags.String("output", "../data/weather", "dossier de destination")
	lookback := flags.Int("lookback-hours", 48, "durée de recherche vers le passé")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("arguments invalides: %w", err)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("arguments inattendus: %v", flags.Args())
	}
	if (*date == "") != (*cycle == "") {
		return options{}, errors.New("-date et -cycle doivent être fournis ensemble")
	}
	if *lookback <= 0 || *lookback > 384 {
		return options{}, errors.New("-lookback-hours doit être compris entre 1 et 384")
	}
	forecastHours, err := parseForecastHours(*hours)
	if err != nil {
		return options{}, err
	}
	if strings.TrimSpace(*output) == "" {
		return options{}, errors.New("-output ne doit pas être vide")
	}
	if *date != "" {
		cycleValue, err := strconv.Atoi(*cycle)
		if err != nil {
			return options{}, fmt.Errorf("cycle %q invalide: %w", *cycle, err)
		}
		for _, forecastHour := range forecastHours {
			if _, err := weather.NewGFSForecastRequest(*date, cycleValue, forecastHour); err != nil {
				return options{}, err
			}
		}
	}
	return options{date: *date, cycle: *cycle, forecastHours: forecastHours, outputDirectory: *output, lookbackHours: *lookback}, nil
}

func candidateRuns(options options, now time.Time) []weather.GFSForecastRequest {
	if options.date != "" {
		cycle, _ := strconv.Atoi(options.cycle)
		request, _ := weather.NewGFSForecastRequest(options.date, cycle, 0)
		return []weather.GFSForecastRequest{request}
	}

	latest := now.Truncate(6 * time.Hour)
	count := options.lookbackHours/6 + 1
	candidates := make([]weather.GFSForecastRequest, 0, count)
	for offset := 0; offset < count; offset++ {
		run := latest.Add(-time.Duration(offset) * 6 * time.Hour)
		request, err := weather.NewGFSForecastRequest(run.Format("20060102"), run.Hour(), 0)
		if err == nil {
			candidates = append(candidates, request)
		}
	}
	return candidates
}

func candidateLabel(request weather.GFSForecastRequest) string {
	return fmt.Sprintf("%s %02dZ", request.Date.Format("2006-01-02"), request.Cycle)
}

func parseForecastHours(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	hours := make([]int, 0, len(parts))
	seen := make(map[int]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		hour, err := strconv.Atoi(part)
		if part == "" || err != nil {
			return nil, fmt.Errorf("échéance %q invalide", part)
		}
		if _, exists := seen[hour]; exists {
			return nil, fmt.Errorf("échéance f%03d présente plusieurs fois", hour)
		}
		seen[hour] = struct{}{}
		hours = append(hours, hour)
	}
	sort.Ints(hours)
	for _, hour := range hours {
		if hour < 0 || hour > 384 {
			return nil, fmt.Errorf("échéance f%03d hors limites [f000, f384]", hour)
		}
	}
	return hours, nil
}

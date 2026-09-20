package weather

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// GFSDownloader is the part of the NOAA downloader needed by a refresh.
type GFSDownloader interface {
	Download(context.Context, GFSForecastRequest, string) (string, error)
}

type GFSRefreshOptions struct {
	ForecastHours   []int
	OutputDirectory string
	LookbackHours   int
	Now             time.Time
}

type GFSRefreshResult struct {
	Run   GFSForecastRequest
	Files []string
}

// RefreshLatestGFS downloads the newest available run, falling back to older
// six-hour runs when NOAA has not published the newest one yet.
func RefreshLatestGFS(ctx context.Context, downloader GFSDownloader, options GFSRefreshOptions) (GFSRefreshResult, error) {
	if ctx == nil {
		return GFSRefreshResult{}, errors.New("weather: contexte requis")
	}
	if downloader == nil {
		return GFSRefreshResult{}, errors.New("weather: téléchargeur requis")
	}
	if options.OutputDirectory == "" {
		return GFSRefreshResult{}, errors.New("weather: dossier de destination requis")
	}
	if options.LookbackHours <= 0 || options.LookbackHours > 384 {
		return GFSRefreshResult{}, errors.New("weather: recherche comprise entre 1 et 384 heures")
	}
	if len(options.ForecastHours) == 0 {
		return GFSRefreshResult{}, errors.New("weather: au moins une échéance est requise")
	}
	hours := append([]int(nil), options.ForecastHours...)
	sort.Ints(hours)
	for _, hour := range hours {
		if hour < 0 || hour > 384 {
			return GFSRefreshResult{}, fmt.Errorf("weather: échéance f%03d hors limites [f000, f384]", hour)
		}
	}

	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	latest := now.Truncate(6 * time.Hour)
	for offset := 0; offset <= options.LookbackHours/6; offset++ {
		runTime := latest.Add(-time.Duration(offset) * 6 * time.Hour)
		candidate, err := NewGFSForecastRequest(runTime.Format(gfsDateLayout), runTime.Hour(), 0)
		if err != nil {
			continue
		}
		files := make([]string, 0, len(hours))
		available := true
		for _, hour := range hours {
			request := candidate
			request.ForecastHour = hour
			path, err := downloader.Download(ctx, request, options.OutputDirectory)
			if err != nil {
				available = false
				break
			}
			files = append(files, path)
		}
		if available {
			return GFSRefreshResult{Run: candidate, Files: files}, nil
		}
	}

	return GFSRefreshResult{}, fmt.Errorf("weather: aucun run GFS disponible dans les %d dernières heures", options.LookbackHours)
}

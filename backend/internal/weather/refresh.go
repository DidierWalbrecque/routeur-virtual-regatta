package weather

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	Run          GFSForecastRequest
	Files        []string
	RemovedFiles []string
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
			removedFiles, cleanupErr := RemoveOldGFSFiles(options.OutputDirectory, candidate)
			if cleanupErr != nil {
				return GFSRefreshResult{}, fmt.Errorf("weather: nettoyage des anciens GRIB impossible: %w", cleanupErr)
			}
			return GFSRefreshResult{Run: candidate, Files: files, RemovedFiles: removedFiles}, nil
		}
	}

	return GFSRefreshResult{}, fmt.Errorf("weather: aucun run GFS disponible dans les %d dernières heures", options.LookbackHours)
}

// RemoveOldGFSFiles removes recognized GRIB files belonging to older runs.
// Files are removed only after a complete run has been downloaded successfully.
func RemoveOldGFSFiles(outputDirectory string, current GFSForecastRequest) ([]string, error) {
	if strings.TrimSpace(outputDirectory) == "" {
		return nil, errors.New("weather: dossier de nettoyage requis")
	}
	entries, err := os.ReadDir(outputDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lecture du dossier %q impossible: %w", outputDirectory, err)
	}
	currentKey := current.Date.Format(gfsDateLayout) + fmt.Sprintf("%02d", current.Cycle)
	removed := make([]string, 0)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		runKey, ok := gfsRunKey(entry.Name())
		if !ok || runKey == currentKey {
			continue
		}
		path := filepath.Join(outputDirectory, entry.Name())
		if err := os.Remove(path); err != nil {
			return removed, fmt.Errorf("suppression de %q impossible: %w", path, err)
		}
		removed = append(removed, path)
	}
	return removed, nil
}

func gfsRunKey(name string) (string, bool) {
	if !strings.HasPrefix(name, "gfs.") || !strings.HasSuffix(name, ".grb2") {
		return "", false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "gfs."), ".grb2"), ".")
	if len(parts) != 2 || len(parts[0]) != 10 || len(parts[1]) != 4 || parts[1][0] != 'f' {
		return "", false
	}
	if _, err := time.Parse("2006010215", parts[0]); err != nil {
		return "", false
	}
	hour, err := strconv.Atoi(parts[1][1:])
	if err != nil || hour < 0 || hour > 384 {
		return "", false
	}
	return parts[0], true
}

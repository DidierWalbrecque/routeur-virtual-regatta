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
	"sync"
	"time"
)

const (
	gfsForecastStepHours = 3
	gfsMaxForecastHour   = 384
	// gfsPublicationLag : délai après le cycle avant que NOAA publie ses premières échéances.
	gfsPublicationLag = 4 * time.Hour
)

// DefaultGFSForecastHours retourne la cadence native du GFS 0,25° : f000 à f384 toutes les 3 heures.
func DefaultGFSForecastHours() []int {
	hours := make([]int, 0, gfsMaxForecastHour/gfsForecastStepHours+1)
	for hour := 0; hour <= gfsMaxForecastHour; hour += gfsForecastStepHours {
		hours = append(hours, hour)
	}
	return hours
}

// GFSRunProgress décrit l'avancement du téléchargement d'un run en cours de publication.
type GFSRunProgress struct {
	Run        GFSForecastRequest
	Downloaded int
	Expected   int
	// Files contient les échéances déjà téléchargées, dans l'ordre.
	Files []string
}

// Complete indique que toutes les échéances attendues sont présentes.
func (progress GFSRunProgress) Complete() bool {
	return progress.Expected > 0 && progress.Downloaded == progress.Expected
}

// GFSWatcherStatus est un instantané de l'état du watcher.
type GFSWatcherStatus struct {
	Active    GFSRefreshResult
	Target    GFSRunProgress
	HasTarget bool
	Busy      bool
	Waiting   string
	LastError string
	Updated   time.Time
}

// GFSWatcher garde un run GFS complet utilisable (le run actif) pendant que le
// run le plus récent se télécharge au fil de sa publication par NOAA. Le run
// actif ne change qu'une fois le run cible entièrement téléchargé.
type GFSWatcher struct {
	// tickMu sérialise les passages ; mu ne protège que l'état, jamais les téléchargements.
	tickMu     sync.Mutex
	mu         sync.Mutex
	downloader GFSDownloader
	directory  string
	hours      []int
	now        func() time.Time
	active     GFSRefreshResult
	target     GFSRunProgress
	hasTarget  bool
	busy       bool
	waiting    string
	lastError  string
	updated    time.Time
}

// NewGFSWatcher construit un watcher et adopte le run complet déjà présent sur disque.
func NewGFSWatcher(downloader GFSDownloader, directory string, hours []int, now func() time.Time) *GFSWatcher {
	if now == nil {
		now = time.Now
	}
	watcher := &GFSWatcher{
		downloader: downloader,
		directory:  directory,
		hours:      sortedGFSHours(hours),
		now:        now,
	}
	active, err := CompleteGFSRun(directory, watcher.hours)
	if err != nil {
		watcher.lastError = err.Error()
	}
	watcher.active = active
	return watcher
}

// SetForecastHours change la liste des échéances à télécharger pour les prochains passages.
func (watcher *GFSWatcher) SetForecastHours(hours []int) error {
	if len(hours) == 0 {
		return errors.New("weather: au moins une échéance est requise")
	}
	for _, hour := range hours {
		if hour < 0 || hour > gfsMaxForecastHour {
			return fmt.Errorf("weather: échéance f%03d hors limites [f000, f%03d]", hour, gfsMaxForecastHour)
		}
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	watcher.hours = sortedGFSHours(hours)
	return nil
}

// Status retourne l'état courant sans rien télécharger.
func (watcher *GFSWatcher) Status() GFSWatcherStatus {
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	return watcher.statusLocked()
}

// Tick télécharge les échéances déjà publiées du run le plus récent (cycle 00,
// 06, 12 ou 18 commencé). NOAA publie les échéances dans l'ordre : la première
// échéance absente arrête le passage, sans erreur, et sera retentée plus tard.
// Quand toutes les échéances sont présentes, ce run devient le run actif et les
// runs plus anciens sont supprimés. Un run plus récent partiel est conservé.
func (watcher *GFSWatcher) Tick(ctx context.Context) (GFSWatcherStatus, []string, error) {
	if ctx == nil {
		return GFSWatcherStatus{}, nil, errors.New("weather: contexte requis")
	}
	watcher.tickMu.Lock()
	defer watcher.tickMu.Unlock()

	watcher.mu.Lock()
	watcher.busy = true
	hours := append([]int(nil), watcher.hours...)
	watcher.mu.Unlock()

	removed, err := watcher.passage(ctx, hours)

	watcher.mu.Lock()
	watcher.busy = false
	status := watcher.statusLocked()
	watcher.mu.Unlock()
	return status, removed, err
}

// passage télécharge sans tenir mu, puis met à jour l'état sous mu.
func (watcher *GFSWatcher) passage(ctx context.Context, hours []int) ([]string, error) {
	now := watcher.now().UTC()
	// Le run ciblé est le dernier cycle publié, pas le dernier cycle commencé.
	cycleTime := now.Add(-gfsPublicationLag).Truncate(6 * time.Hour)
	candidate, err := NewGFSForecastRequest(cycleTime.Format(gfsDateLayout), cycleTime.Hour(), 0)
	if err != nil {
		return nil, err
	}

	progress, files, downloadErr := downloadRunPrefix(ctx, watcher.downloader, candidate, hours, watcher.directory)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	watcher.updated = now
	watcher.target = progress
	watcher.hasTarget = true
	watcher.waiting = ""
	if downloadErr != nil {
		watcher.waiting = downloadErr.Error()
	}
	watcher.lastError = ""

	if !progress.Complete() || gfsRunKeyOf(candidate) <= gfsRunKeyOf(watcher.active.Run) {
		return nil, nil
	}
	removed, err := RemoveOldGFSFiles(watcher.directory, candidate)
	if err != nil {
		watcher.lastError = err.Error()
		return removed, err
	}
	watcher.active = GFSRefreshResult{Run: candidate, Files: files, RemovedFiles: removed}
	return removed, nil
}

// Run exécute Tick immédiatement, puis à chaque intervalle, jusqu'à l'annulation du contexte.
func (watcher *GFSWatcher) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		_, _, _ = watcher.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (watcher *GFSWatcher) statusLocked() GFSWatcherStatus {
	return GFSWatcherStatus{
		Active:    watcher.active,
		Target:    watcher.target,
		HasTarget: watcher.hasTarget,
		Busy:      watcher.busy,
		Waiting:   watcher.waiting,
		LastError: watcher.lastError,
		Updated:   watcher.updated,
	}
}

// downloadRunPrefix télécharge les échéances dans l'ordre et s'arrête à la
// première qui échoue. Les fichiers déjà présents ne sont pas retéléchargés.
func downloadRunPrefix(ctx context.Context, downloader GFSDownloader, run GFSForecastRequest, hours []int, directory string) (GFSRunProgress, []string, error) {
	progress := GFSRunProgress{Run: run, Expected: len(hours)}
	files := make([]string, 0, len(hours))
	for _, hour := range hours {
		request := run
		request.ForecastHour = hour
		path, err := downloader.Download(ctx, request, directory)
		if err != nil {
			return progress, files, fmt.Errorf("échéance f%03d non disponible : %w", hour, err)
		}
		files = append(files, path)
		progress.Downloaded++
		progress.Files = append([]string(nil), files...)
	}
	return progress, files, nil
}

// CompleteGFSRun retourne le run le plus récent présent sur disque qui possède
// toutes les échéances demandées. Un run partiellement publié est ignoré.
// Un répertoire absent ou sans run complet retourne un résultat vide sans erreur.
func CompleteGFSRun(directory string, hours []int) (GFSRefreshResult, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return GFSRefreshResult{}, nil
	}
	if err != nil {
		return GFSRefreshResult{}, fmt.Errorf("weather: lecture du répertoire %q impossible: %w", directory, err)
	}

	filesByRun := map[string]map[int]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		runKey, ok := gfsRunKey(entry.Name())
		if !ok {
			continue
		}
		// gfsRunKey a déjà validé le format "gfs.<run>.f<hhh>.grb2".
		hourText := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "gfs."+runKey+"."), ".grb2")
		hour, err := strconv.Atoi(strings.TrimPrefix(hourText, "f"))
		if err != nil {
			continue
		}
		if filesByRun[runKey] == nil {
			filesByRun[runKey] = map[int]string{}
		}
		filesByRun[runKey][hour] = filepath.Join(directory, entry.Name())
	}

	keys := make([]string, 0, len(filesByRun))
	for key := range filesByRun {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		paths := make([]string, 0, len(hours))
		complete := len(hours) > 0
		for _, hour := range hours {
			path, found := filesByRun[key][hour]
			if !found {
				complete = false
				break
			}
			paths = append(paths, path)
		}
		if !complete {
			continue
		}
		cycle, err := strconv.Atoi(key[8:])
		if err != nil {
			return GFSRefreshResult{}, fmt.Errorf("weather: cycle GFS %q invalide: %w", key, err)
		}
		run, err := NewGFSForecastRequest(key[:8], cycle, 0)
		if err != nil {
			return GFSRefreshResult{}, err
		}
		return GFSRefreshResult{Run: run, Files: paths}, nil
	}
	return GFSRefreshResult{}, nil
}

func gfsRunKeyOf(request GFSForecastRequest) string {
	if request.Date.IsZero() {
		return ""
	}
	return request.Date.Format(gfsDateLayout) + fmt.Sprintf("%02d", request.Cycle)
}

func sortedGFSHours(hours []int) []int {
	sorted := append([]int(nil), hours...)
	sort.Ints(sorted)
	return sorted
}

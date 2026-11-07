package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

const (
	gribDirectory     = "../data/weather"
	gribWatchInterval = 5 * time.Minute
)

// gribWatcher garde le dernier run GFS complet actif et télécharge le run suivant au fil de sa publication.
var gribWatcher = weather.NewGFSWatcher(weather.NewNOAAGFSDownloader(), gribDirectory, weather.DefaultGFSForecastHours(), time.Now)

// StartGRIBWatcher lance en arrière-plan le téléchargement progressif du run GFS le plus récent.
func StartGRIBWatcher(ctx context.Context) {
	go gribWatcher.Run(ctx, gribWatchInterval)
}

type weatherRefreshRequest struct {
	ForecastHours []int `json:"forecastHours"`
	// LookbackHours est accepté pour compatibilité ; le watcher cible le run le plus récent publié.
	LookbackHours int `json:"lookbackHours"`
}

type weatherTargetResponse struct {
	Run        string `json:"run"`
	Downloaded int    `json:"downloaded"`
	Expected   int    `json:"expected"`
	Complete   bool   `json:"complete"`
}

type weatherStatusResponse struct {
	Run          string                 `json:"run"`
	Files        []string               `json:"files"`
	RemovedFiles []string               `json:"removedFiles"`
	Refreshed    string                 `json:"refreshed"`
	Target       *weatherTargetResponse `json:"target,omitempty"`
	Waiting      string                 `json:"waiting,omitempty"`
	LastError    string                 `json:"lastError,omitempty"`
}

// weatherRefreshHandler déclenche un passage du watcher et renvoie le run actif (toujours complet).
func weatherRefreshHandler(response http.ResponseWriter, request *http.Request) {
	var payload weatherRefreshRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(response, http.StatusBadRequest, "requête JSON invalide: "+err.Error())
		return
	}
	if len(payload.ForecastHours) > 0 {
		if err := gribWatcher.SetForecastHours(payload.ForecastHours); err != nil {
			writeJSONError(response, http.StatusBadRequest, "échéances GRIB invalides: "+err.Error())
			return
		}
	}

	status, removed, err := gribWatcher.Tick(request.Context())
	if err != nil && request.Context().Err() != nil {
		writeJSONError(response, http.StatusRequestTimeout, "actualisation interrompue: "+err.Error())
		return
	}
	writeJSON(response, http.StatusOK, weatherStatusFrom(status, removed, err))
}

// weatherCurrentHandler renvoie le run actif, sans rien télécharger.
func weatherCurrentHandler(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, weatherStatusFrom(gribWatcher.Status(), nil, nil))
}

func weatherStatusFrom(status weather.GFSWatcherStatus, removed []string, err error) weatherStatusResponse {
	if removed == nil {
		removed = []string{}
	}
	files := status.Active.Files
	if files == nil {
		files = []string{}
	}
	response := weatherStatusResponse{
		Run:          runLabel(status.Active.Run),
		Files:        files,
		RemovedFiles: removed,
		Refreshed:    status.Updated.Format(time.RFC3339),
		Waiting:      status.Waiting,
		LastError:    status.LastError,
	}
	if err != nil {
		response.LastError = err.Error()
	}
	if status.HasTarget {
		response.Target = &weatherTargetResponse{
			Run:        runLabel(status.Target.Run),
			Downloaded: status.Target.Downloaded,
			Expected:   status.Target.Expected,
			Complete:   status.Target.Complete(),
		}
	}
	return response
}

func runLabel(request weather.GFSForecastRequest) string {
	if request.Date.IsZero() {
		return ""
	}
	return request.Date.Format("20060102") + formatCycle(request.Cycle)
}

func formatCycle(cycle int) string {
	return map[int]string{0: "00", 6: "06", 12: "12", 18: "18"}[cycle]
}

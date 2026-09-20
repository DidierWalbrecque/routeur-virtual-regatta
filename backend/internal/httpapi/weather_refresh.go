package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type weatherRefreshRequest struct {
	ForecastHours []int `json:"forecastHours"`
	LookbackHours int   `json:"lookbackHours"`
}

type weatherRefreshResponse struct {
	Run       string   `json:"run"`
	Files     []string `json:"files"`
	Refreshed string   `json:"refreshed"`
}

func weatherRefreshHandler(response http.ResponseWriter, request *http.Request) {
	var payload weatherRefreshRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(response, http.StatusBadRequest, "requête JSON invalide: "+err.Error())
		return
	}
	if len(payload.ForecastHours) == 0 {
		payload.ForecastHours = []int{0, 3, 6, 9, 12, 15, 18, 21, 24}
	}
	if payload.LookbackHours == 0 {
		payload.LookbackHours = 48
	}

	result, err := weather.RefreshLatestGFS(request.Context(), weather.NewNOAAGFSDownloader(), weather.GFSRefreshOptions{
		ForecastHours: payload.ForecastHours, OutputDirectory: "../data/weather", LookbackHours: payload.LookbackHours,
	})
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "actualisation météo impossible: "+err.Error())
		return
	}
	writeJSON(response, http.StatusOK, weatherRefreshResponse{
		Run: result.Run.Date.Format("20060102") + formatCycle(result.Run.Cycle), Files: result.Files, Refreshed: time.Now().UTC().Format(time.RFC3339),
	})
}

func formatCycle(cycle int) string {
	return map[int]string{0: "00", 6: "06", 12: "12", 18: "18"}[cycle]
}

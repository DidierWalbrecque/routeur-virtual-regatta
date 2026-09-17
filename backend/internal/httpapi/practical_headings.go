package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type practicalHeadingsRequest struct {
	WeatherFiles              []string                            `json:"weatherFiles"`
	PolarPath                 string                              `json:"polarPath"`
	Position                  positionRequest                     `json:"position"`
	Destination               positionRequest                     `json:"destination"`
	Instant                   string                              `json:"instant"`
	DurationMinutes           float64                             `json:"durationMinutes"`
	StepMinutes               float64                             `json:"stepMinutes"`
	Headings                  []float64                           `json:"headings"`
	Owned                     []string                            `json:"owned"`
	FixedSailID               polar.SailID                        `json:"fixedSailID"`
	WeatherVariations         []routing.PracticalWeatherVariation `json:"weatherVariations"`
	MaxScenarioProgressLossNM float64                             `json:"maxScenarioProgressLossNM"`
	RequireRobustScenarios    bool                                `json:"requireRobustScenarios"`
}

type positionRequest struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	VirtualRegatta string  `json:"virtualRegatta"`
}

type practicalHeadingsResponse struct {
	Candidates []practicalCandidateResponse `json:"candidates"`
}

type practicalCandidateResponse struct {
	Heading                float64         `json:"heading"`
	ProgressNM             float64         `json:"progressNM"`
	RemainingDistanceNM    float64         `json:"remainingDistanceNM"`
	StabilityScore         float64         `json:"stabilityScore"`
	WorstCaseProgressNM    float64         `json:"worstCaseProgressNM"`
	ScenarioProgressLossNM float64         `json:"scenarioProgressLossNM"`
	Robust                 bool            `json:"robust"`
	NextPosition           positionRequest `json:"nextPosition"`
	NextInstant            time.Time       `json:"nextInstant"`
	Sail                   string          `json:"sail"`
	SpeedKnots             float64         `json:"speedKnots"`
}

func practicalHeadingsHandler(response http.ResponseWriter, request *http.Request) {
	var payload practicalHeadingsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(response, http.StatusBadRequest, "requête JSON invalide: "+err.Error())
		return
	}

	instant, err := time.Parse(time.RFC3339, payload.Instant)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "instant RFC3339 invalide: "+err.Error())
		return
	}
	if len(payload.WeatherFiles) == 0 || strings.TrimSpace(payload.PolarPath) == "" {
		writeJSONError(response, http.StatusBadRequest, "weatherFiles et polarPath sont requis")
		return
	}
	owned, err := equipment.ParseSet(strings.Join(payload.Owned, ","))
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "équipements invalides: "+err.Error())
		return
	}
	headings := make([]navigation.Heading, len(payload.Headings))
	for index, heading := range payload.Headings {
		headings[index] = navigation.Heading(heading)
	}
	weatherProvider, err := weather.LoadGRIB2WeatherProvider(payload.WeatherFiles...)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "chargement météo impossible: "+err.Error())
		return
	}
	boatPolar, err := polar.LoadFile(payload.PolarPath)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "chargement polaire impossible: "+err.Error())
		return
	}
	position, err := parsePositionRequest(payload.Position)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "position de départ invalide: "+err.Error())
		return
	}
	destination, err := parsePositionRequest(payload.Destination)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "destination invalide: "+err.Error())
		return
	}
	candidates, err := routing.EvaluatePracticalHeadingCandidates(
		request.Context(), weatherProvider, boatPolar, routing.PracticalHeadingInput{
			Position:    position,
			Destination: destination,
			Instant:     instant, Duration: time.Duration(payload.DurationMinutes * float64(time.Minute)), StepDuration: time.Duration(payload.StepMinutes * float64(time.Minute)),
			Owned: owned, FixedSailID: payload.FixedSailID, Headings: headings, WeatherVariations: payload.WeatherVariations,
			MaxScenarioProgressLoss: navigation.NauticalMiles(payload.MaxScenarioProgressLossNM), RequireRobustScenarios: payload.RequireRobustScenarios,
		},
	)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "calcul pratique impossible: "+err.Error())
		return
	}
	result := practicalHeadingsResponse{Candidates: make([]practicalCandidateResponse, 0, len(candidates))}
	for _, candidate := range candidates {
		lastStep := candidate.Simulation.Steps[len(candidate.Simulation.Steps)-1]
		result.Candidates = append(result.Candidates, practicalCandidateResponse{
			Heading: float64(candidate.Heading), ProgressNM: float64(candidate.Progress), RemainingDistanceNM: float64(candidate.RemainingDistance), StabilityScore: candidate.StabilityScore,
			WorstCaseProgressNM: float64(candidate.WorstCaseProgress), ScenarioProgressLossNM: float64(candidate.ScenarioProgressLoss), Robust: candidate.Robust,
			NextPosition: positionRequest{Latitude: float64(candidate.Simulation.NextPosition.Latitude), Longitude: float64(candidate.Simulation.NextPosition.Longitude)},
			NextInstant:  candidate.Simulation.NextInstant, Sail: lastStep.BestSail.SailName, SpeedKnots: float64(lastStep.BestSail.Speed),
		})
	}
	writeJSON(response, http.StatusOK, result)
}

func parsePositionRequest(request positionRequest) (navigation.Position, error) {
	if strings.TrimSpace(request.VirtualRegatta) != "" {
		return navigation.ParseVirtualRegattaPosition(request.VirtualRegatta)
	}
	position := navigation.Position{Latitude: navigation.Latitude(request.Latitude), Longitude: navigation.Longitude(request.Longitude)}
	if err := position.Validate(); err != nil {
		return navigation.Position{}, err
	}
	return position, nil
}

func writeJSONError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}
func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

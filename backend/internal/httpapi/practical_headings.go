package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
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
	Mode                      string                              `json:"mode"`
}

type positionRequest struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	VirtualRegatta string  `json:"virtualRegatta"`
}

type practicalHeadingsResponse struct {
	Position    positionRequest              `json:"position"`
	Destination positionRequest              `json:"destination"`
	Candidates  []practicalCandidateResponse `json:"candidates"`
	Isochrones  []isochroneFrontierResponse  `json:"isochrones"`
	Route       []routePointResponse         `json:"route"`
	Performance performanceResponse          `json:"performance"`
}

type performanceResponse struct {
	TotalMilliseconds     float64 `json:"totalMilliseconds"`
	WeatherMilliseconds   float64 `json:"weatherMilliseconds"`
	PolarMilliseconds     float64 `json:"polarMilliseconds"`
	LandMilliseconds      float64 `json:"landMilliseconds"`
	PracticalMilliseconds float64 `json:"practicalMilliseconds"`
	IsochroneMilliseconds float64 `json:"isochroneMilliseconds"`
	CacheHits             int     `json:"cacheHits"`
	CacheMisses           int     `json:"cacheMisses"`
}

const landGeoJSONPath = "../data/land/natural-earth-10m-land.geojson"

type calculationProfile struct {
	IsochroneStep     time.Duration
	HeadingSpacing    float64
	CellSize          navigation.NauticalMiles
	MaxCandidates     int
	IncludeIsochrones bool
}

func calculationProfileFor(mode string) (calculationProfile, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "precise":
		return calculationProfile{IsochroneStep: 10 * time.Minute, HeadingSpacing: 15, CellSize: 0.5, MaxCandidates: 100, IncludeIsochrones: true}, nil
	case "quick":
		return calculationProfile{IsochroneStep: 15 * time.Minute, HeadingSpacing: 30, CellSize: 1, MaxCandidates: 50, IncludeIsochrones: true}, nil
	default:
		return calculationProfile{}, fmt.Errorf("mode de calcul %q invalide: quick ou precise attendu", mode)
	}
}

type isochroneFrontierResponse struct {
	Instant   time.Time         `json:"instant"`
	Positions []positionRequest `json:"positions"`
	Contour   []positionRequest `json:"contour"`
}
type routePointResponse struct {
	Position positionRequest `json:"position"`
	Heading  float64         `json:"heading"`
	Sail     string          `json:"sail"`
	Instant  time.Time       `json:"instant"`
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
	totalStarted := time.Now()
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
	profile, err := calculationProfileFor(payload.Mode)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
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
	weatherStarted := time.Now()
	weatherProvider, weatherStats, err := loadCachedWeatherProvider(payload.WeatherFiles...)
	weatherMilliseconds := elapsedMilliseconds(weatherStarted)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "chargement météo impossible: "+err.Error())
		return
	}
	polarStarted := time.Now()
	boatPolar, polarHit, err := loadCachedPolar(payload.PolarPath)
	polarMilliseconds := elapsedMilliseconds(polarStarted)
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
	landStarted := time.Now()
	landConstraint, landHit, err := loadCachedLand(landGeoJSONPath)
	landMilliseconds := elapsedMilliseconds(landStarted)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "données terrestres indisponibles: "+err.Error())
		return
	}
	landConstraint, err = landConstraint.WithCoastalMargin(navigation.NauticalMiles(2))
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "marge côtière impossible: "+err.Error())
		return
	}
	for label, point := range map[string]navigation.Position{"départ": position, "destination": destination} {
		onLand, err := landConstraint.Contains(point)
		if err != nil {
			writeJSONError(response, http.StatusBadRequest, "vérification terrestre impossible pour le "+label+": "+err.Error())
			return
		}
		if onLand {
			writeJSONError(response, http.StatusBadRequest, "le point de "+label+" se trouve sur une terre ou la côte")
			return
		}
	}
	practicalStarted := time.Now()
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
	practicalMilliseconds := elapsedMilliseconds(practicalStarted)
	result := practicalHeadingsResponse{
		Position:    positionRequest{Latitude: float64(position.Latitude), Longitude: float64(position.Longitude)},
		Destination: positionRequest{Latitude: float64(destination.Latitude), Longitude: float64(destination.Longitude)},
		Candidates:  make([]practicalCandidateResponse, 0, len(candidates)),
		Isochrones:  make([]isochroneFrontierResponse, 0),
		Route:       make([]routePointResponse, 0),
	}
	result.Performance.CacheHits = weatherStats.Hits
	result.Performance.CacheMisses = weatherStats.Misses
	if polarHit {
		result.Performance.CacheHits++
	} else {
		result.Performance.CacheMisses++
	}
	if landHit {
		result.Performance.CacheHits++
	} else {
		result.Performance.CacheMisses++
	}
	result.Performance.WeatherMilliseconds = weatherMilliseconds
	result.Performance.PolarMilliseconds = polarMilliseconds
	result.Performance.LandMilliseconds = landMilliseconds
	for _, candidate := range candidates {
		lastStep := candidate.Simulation.Steps[len(candidate.Simulation.Steps)-1]
		result.Candidates = append(result.Candidates, practicalCandidateResponse{
			Heading: float64(candidate.Heading), ProgressNM: float64(candidate.Progress), RemainingDistanceNM: float64(candidate.RemainingDistance), StabilityScore: candidate.StabilityScore,
			WorstCaseProgressNM: float64(candidate.WorstCaseProgress), ScenarioProgressLossNM: float64(candidate.ScenarioProgressLoss), Robust: candidate.Robust,
			NextPosition: positionRequest{Latitude: float64(candidate.Simulation.NextPosition.Latitude), Longitude: float64(candidate.Simulation.NextPosition.Longitude)},
			NextInstant:  candidate.Simulation.NextInstant, Sail: lastStep.BestSail.SailName, SpeedKnots: float64(lastStep.BestSail.Speed),
		})
	}
	result.Performance.PracticalMilliseconds = practicalMilliseconds
	if !profile.IncludeIsochrones {
		result.Performance.TotalMilliseconds = elapsedMilliseconds(totalStarted)
		writeJSON(response, http.StatusOK, result)
		return
	}
	isochroneStep := profile.IsochroneStep
	isochroneStarted := time.Now()
	layerCount := int(math.Ceil(time.Duration(payload.DurationMinutes*float64(time.Minute)).Minutes() / isochroneStep.Minutes()))
	if layerCount > 0 {
		search, err := routing.BuildIsochroneSearch(request.Context(), weatherProvider, boatPolar, routing.IsochroneSearchInput{
			Position: position, Destination: destination, Instant: instant, StepDuration: isochroneStep, LayerCount: layerCount,
			Owned: owned, HeadingSpacing: profile.HeadingSpacing, CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
			ArrivalRadius: navigation.NauticalMiles(1), InitialStamina: simulation.StaminaState{Level: 100},
			Constraints: []routing.Constraint{landConstraint},
		})
		if err != nil {
			writeJSONError(response, http.StatusBadRequest, "calcul des isochrones impossible: "+err.Error())
			return
		}
		for _, frontier := range search.Frontiers {
			layer := isochroneFrontierResponse{Instant: frontier.Instant, Positions: make([]positionRequest, 0, len(frontier.Nodes)), Contour: make([]positionRequest, 0)}
			for _, node := range frontier.Nodes {
				layer.Positions = append(layer.Positions, positionRequest{Latitude: float64(node.Candidate.Step.NextPosition.Latitude), Longitude: float64(node.Candidate.Step.NextPosition.Longitude)})
			}
			contour, err := frontier.Contour(position, 36)
			if err != nil {
				writeJSONError(response, http.StatusBadRequest, "contour d’isochrone impossible: "+err.Error())
				return
			}
			for _, contourPosition := range contour {
				layer.Contour = append(layer.Contour, positionRequest{Latitude: float64(contourPosition.Latitude), Longitude: float64(contourPosition.Longitude)})
			}
			result.Isochrones = append(result.Isochrones, layer)
		}
		bestRoute, err := search.ReconstructBestRoute()
		if err == nil {
			result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(bestRoute.Origin.Latitude), Longitude: float64(bestRoute.Origin.Longitude)}, Instant: bestRoute.StartedAt})
			for _, routeStep := range bestRoute.Steps {
				result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(routeStep.Node.Candidate.Step.NextPosition.Latitude), Longitude: float64(routeStep.Node.Candidate.Step.NextPosition.Longitude)}, Heading: float64(routeStep.Node.Candidate.Heading), Sail: routeStep.Node.Candidate.Step.BestSail.SailName, Instant: routeStep.Node.Candidate.Step.NextInstant})
			}
		}
		result.Performance.IsochroneMilliseconds = elapsedMilliseconds(isochroneStarted)
	}
	result.Performance.TotalMilliseconds = elapsedMilliseconds(totalStarted)
	writeJSON(response, http.StatusOK, result)
}

func elapsedMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
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

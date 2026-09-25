package httpapi

import (
	"context"
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
	Reached     bool                         `json:"destinationReached"`
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
	IsochroneDurationMin  float64 `json:"isochroneDurationMinutes"`
}

var landGeoJSONPath = "../data/land/natural-earth-10m-land.geojson"

var (
	loadWeatherProviderForRequest = loadCachedWeatherProvider
	loadPolarForRequest           = loadCachedPolar
	loadLandForRequest            = loadCachedLand
	evaluatePracticalHeadings     = routing.EvaluatePracticalHeadingCandidates
	buildIsochroneSearch          = routing.BuildIsochroneSearch
)

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

func adaptiveIsochroneDuration(distance navigation.NauticalMiles, requested time.Duration, maxSpeed polar.BoatSpeed) time.Duration {
	if requested <= 0 || maxSpeed <= 0 || distance <= 0 {
		return requested
	}
	const (
		conservativeSpeedRatio = 0.6
		minimumBuffer          = 60 * time.Minute
	)
	estimated := time.Duration(float64(distance) / (float64(maxSpeed) * conservativeSpeedRatio) * float64(time.Hour))
	estimated += minimumBuffer
	if estimated < minimumBuffer {
		estimated = minimumBuffer
	}
	if estimated > requested {
		return requested
	}
	return estimated
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
	Terminal bool            `json:"terminal"`
}

type availableRouteStep struct {
	Position navigation.Position
	Heading  navigation.Heading
	Sail     string
	Instant  time.Time
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
	runPracticalHeadings(response, request, nil)
}

func practicalHeadingsStreamHandler(response http.ResponseWriter, request *http.Request) {
	runPracticalHeadings(response, request, &practicalHeadingsStream{})
}

func runPracticalHeadings(
	response http.ResponseWriter,
	request *http.Request,
	stream *practicalHeadingsStream,
) {
	totalStarted := time.Now()
	calculationContext := request.Context()
	var cancelCalculation context.CancelFunc
	if stream != nil {
		calculationContext, cancelCalculation = context.WithCancel(calculationContext)
		defer cancelCalculation()
	}
	var streamCallbackError error
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
	weatherProvider, weatherStats, err := loadWeatherProviderForRequest(payload.WeatherFiles...)
	weatherMilliseconds := elapsedMilliseconds(weatherStarted)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "chargement météo impossible: "+err.Error())
		return
	}
	polarStarted := time.Now()
	boatPolar, polarHit, err := loadPolarForRequest(payload.PolarPath)
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
	landConstraint, landHit, err := loadLandForRequest(landGeoJSONPath)
	landMilliseconds := elapsedMilliseconds(landStarted)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "données terrestres indisponibles: "+err.Error())
		return
	}
	arrivalLandConstraint := landConstraint
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
	candidates, err := evaluatePracticalHeadings(
		calculationContext, weatherProvider, boatPolar, routing.PracticalHeadingInput{
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
	if payload.StepMinutes > 0 {
		isochroneStep = time.Duration(payload.StepMinutes * float64(time.Minute))
	}
	isochroneStarted := time.Now()
	requestedDuration := time.Duration(payload.DurationMinutes * float64(time.Minute))
	distance, err := navigation.DistanceBetween(position, destination)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "distance de destination impossible: "+err.Error())
		return
	}
	calculationDuration := adaptiveIsochroneDuration(distance, requestedDuration, boatPolar.MaxSpeed)
	// A land-constrained route may need a long detour even when the direct
	// distance is short. Keep the requested horizon in that case so the search
	// can continue around obstacles instead of producing a false final error.
	if landConstraint != nil {
		calculationDuration = requestedDuration
	}
	result.Performance.IsochroneDurationMin = calculationDuration.Minutes()
	layerCount := int(math.Ceil(calculationDuration.Minutes() / isochroneStep.Minutes()))
	if layerCount > 0 {
		if stream != nil {
			if err := stream.start(response, practicalHeadingsStartEvent{
				Position:    result.Position,
				Destination: result.Destination,
				StartedAt:   instant,
				TotalLayers: layerCount,
			}); err != nil {
				cancelCalculation()
				return
			}
		}
		progressReporter := routing.IsochroneProgressReporter(nil)
		if stream != nil {
			progressReporter = func(progress routing.IsochroneProgress) {
				frontier, frontierErr := isochroneFrontierResponseFor(position, progress.Frontier)
				if frontierErr != nil {
					streamCallbackError = frontierErr
					cancelCalculation()
					return
				}
				if err := stream.event(response, "frontier", practicalHeadingsFrontierEvent{
					Instant:            frontier.Instant.Format(time.RFC3339Nano),
					Positions:          frontier.Positions,
					Contour:            frontier.Contour,
					CompletedLayers:    progress.CompletedLayers,
					TotalLayers:        progress.TotalLayers,
					Progress:           float64(progress.CompletedLayers) / float64(progress.TotalLayers),
					DestinationReached: progress.Reached,
				}); err != nil {
					streamCallbackError = err
					cancelCalculation()
				}
			}
		}
		search, err := buildIsochroneSearch(calculationContext, weatherProvider, boatPolar, routing.IsochroneSearchInput{
			Position: position, Destination: destination, Instant: instant, StepDuration: isochroneStep, LayerCount: layerCount,
			Owned: owned, HeadingSpacing: profile.HeadingSpacing, CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
			ArrivalRadius: navigation.NauticalMiles(1), InitialStamina: simulation.StaminaState{Level: 100},
			Constraints:        []routing.Constraint{landConstraint},
			ArrivalConstraints: []routing.Constraint{arrivalLandConstraint},
			Progress:           progressReporter,
		})
		if err != nil {
			if stream != nil {
				if streamCallbackError != nil {
					stream.fail(response, "transmission de la progression impossible: "+streamCallbackError.Error(), streamCallbackError)
				} else {
					stream.fail(response, "calcul des isochrones impossible: "+err.Error(), err)
				}
				return
			}
			writeJSONError(response, http.StatusBadRequest, "calcul des isochrones impossible: "+err.Error())
			return
		}
		result.Reached = search.Reached
		for _, frontier := range search.Frontiers {
			layer, err := isochroneFrontierResponseFor(position, frontier)
			if err != nil {
				if stream != nil {
					stream.fail(response, "contour d’isochrone impossible: "+err.Error(), err)
				} else {
					writeJSONError(response, http.StatusBadRequest, "contour d’isochrone impossible: "+err.Error())
				}
				return
			}
			result.Isochrones = append(result.Isochrones, layer)
		}
		if !search.Reached {
			availableSteps, routeErr := reconstructBestAvailableRoute(search)
			if routeErr != nil {
				if stream != nil {
					stream.fail(response, "reconstruction de la meilleure route impossible: "+routeErr.Error(), routeErr)
				} else {
					writeJSONError(response, http.StatusBadRequest, "reconstruction de la meilleure route impossible: "+routeErr.Error())
				}
				return
			}
			result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(search.Origin.Latitude), Longitude: float64(search.Origin.Longitude)}, Instant: search.StartedAt})
			for _, routeStep := range availableSteps {
				result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(routeStep.Position.Latitude), Longitude: float64(routeStep.Position.Longitude)}, Heading: float64(routeStep.Heading), Sail: routeStep.Sail, Instant: routeStep.Instant})
			}
		} else if bestRoute, err := search.ReconstructBestRoute(); err == nil {
			routeSteps := bestRoute.Steps
			if search.Reached && len(routeSteps) > 0 {
				previousPosition := bestRoute.Origin
				if len(routeSteps) > 1 {
					previousPosition = routeSteps[len(routeSteps)-2].Node.Candidate.Step.NextPosition
				}
				lastPosition := routeSteps[len(routeSteps)-1].Node.Candidate.Step.NextPosition
				passesDestination, distanceErr := routeStepPassesDestination(previousPosition, lastPosition, destination)
				if distanceErr != nil {
					if stream != nil {
						stream.fail(response, "vérification du dernier point impossible: "+distanceErr.Error(), distanceErr)
					} else {
						writeJSONError(response, http.StatusBadRequest, "vérification du dernier point impossible: "+distanceErr.Error())
					}
					return
				}
				if passesDestination {
					routeSteps = routeSteps[:len(routeSteps)-1]
				}
			}
			result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(bestRoute.Origin.Latitude), Longitude: float64(bestRoute.Origin.Longitude)}, Instant: bestRoute.StartedAt})
			for _, routeStep := range routeSteps {
				result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(routeStep.Node.Candidate.Step.NextPosition.Latitude), Longitude: float64(routeStep.Node.Candidate.Step.NextPosition.Longitude)}, Heading: float64(routeStep.Node.Candidate.Heading), Sail: routeStep.Node.Candidate.Step.BestSail.SailName, Instant: routeStep.Node.Candidate.Step.NextInstant})
			}
			if search.Reached {
				lastPosition := bestRoute.Origin
				lastInstant := bestRoute.StartedAt
				if len(routeSteps) > 0 {
					lastPosition = routeSteps[len(routeSteps)-1].Node.Candidate.Step.NextPosition
					lastInstant = routeSteps[len(routeSteps)-1].Node.Candidate.Step.NextInstant
				}
				if lastPosition != destination {
					finalSegment := routing.RouteSegment{From: lastPosition, To: destination, StartedAt: lastInstant, FinishedAt: lastInstant.Add(profile.IsochroneStep)}
					allowed, constraintErr := arrivalLandConstraint.Allows(calculationContext, finalSegment)
					if constraintErr != nil {
						if stream != nil {
							stream.fail(response, "vérification de l’arrivée impossible: "+constraintErr.Error(), constraintErr)
						} else {
							writeJSONError(response, http.StatusBadRequest, "vérification de l’arrivée impossible: "+constraintErr.Error())
						}
						return
					}
					if !allowed {
						if stream != nil {
							stream.fail(response, "la dernière approche de la destination traverse une terre ou la côte", nil)
						} else {
							writeJSONError(response, http.StatusBadRequest, "la dernière approche de la destination traverse une terre ou la côte")
						}
						return
					}
				}
				result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(destination.Latitude), Longitude: float64(destination.Longitude)}, Instant: lastInstant.Add(profile.IsochroneStep), Terminal: true})
			}
		}
		result.Performance.IsochroneMilliseconds = elapsedMilliseconds(isochroneStarted)
	}
	result.Performance.TotalMilliseconds = elapsedMilliseconds(totalStarted)
	if stream != nil {
		_ = stream.event(response, "complete", result)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func isochroneFrontierResponseFor(
	origin navigation.Position,
	frontier routing.IsochroneFrontier,
) (isochroneFrontierResponse, error) {
	layer := isochroneFrontierResponse{
		Instant:   frontier.Instant,
		Positions: make([]positionRequest, 0, len(frontier.Nodes)),
		Contour:   make([]positionRequest, 0),
	}
	for _, node := range frontier.Nodes {
		layer.Positions = append(layer.Positions, positionRequest{
			Latitude:  float64(node.Candidate.Step.NextPosition.Latitude),
			Longitude: float64(node.Candidate.Step.NextPosition.Longitude),
		})
	}
	contour, err := frontier.Contour(origin, 36)
	if err != nil {
		return isochroneFrontierResponse{}, err
	}
	for _, contourPosition := range contour {
		layer.Contour = append(layer.Contour, positionRequest{
			Latitude:  float64(contourPosition.Latitude),
			Longitude: float64(contourPosition.Longitude),
		})
	}
	return layer, nil
}

func reconstructBestAvailableRoute(search routing.IsochroneSearch) ([]availableRouteStep, error) {
	if len(search.Frontiers) == 0 {
		return nil, fmt.Errorf("aucun front d’isochrone disponible")
	}
	steps := make([]availableRouteStep, len(search.Frontiers))
	nodeIndex := 0
	for frontierIndex := len(search.Frontiers) - 1; frontierIndex >= 0; frontierIndex-- {
		frontier := search.Frontiers[frontierIndex]
		if nodeIndex < 0 || nodeIndex >= len(frontier.Nodes) {
			return nil, fmt.Errorf("nœud %d absent du front %d", nodeIndex, frontierIndex+1)
		}
		node := frontier.Nodes[nodeIndex]
		steps[frontierIndex] = availableRouteStep{
			Position: node.Candidate.Step.NextPosition,
			Heading:  node.Candidate.Heading,
			Sail:     node.Candidate.Step.BestSail.SailName,
			Instant:  node.Candidate.Step.NextInstant,
		}
		nodeIndex = node.ParentIndex
	}
	return steps, nil
}

func routeStepPassesDestination(from, to, destination navigation.Position) (bool, error) {
	fromToDestination, err := navigation.DistanceBetween(from, destination)
	if err != nil {
		return false, err
	}
	destinationToTo, err := navigation.DistanceBetween(destination, to)
	if err != nil {
		return false, err
	}
	fromToTo, err := navigation.DistanceBetween(from, to)
	if err != nil {
		return false, err
	}
	const (
		arrivalRadiusNM     = 1.0
		progressToleranceNM = 0.05
	)
	return destinationToTo <= arrivalRadiusNM &&
		float64(fromToDestination)+progressToleranceNM < float64(fromToTo), nil
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

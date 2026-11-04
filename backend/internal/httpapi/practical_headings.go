package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	// Réglages découplés de l'intégration Virtual Regatta. PositionSource et
	// StaminaSource valent "vr" ou "manual" ; ManualStamina (0..100) sert quand
	// StaminaSource vaut "manual".
	PositionSource            string                              `json:"positionSource"`
	StaminaSource             string                              `json:"staminaSource"`
	ManualStamina             *float64                            `json:"manualStamina"`
	UseCourseGates            bool                                `json:"useCourseGates"`
	// UseVirtualRegattaState : alias hérité activant les trois réglages ci-dessus
	// sur Virtual Regatta lorsqu'il est seul présent.
	UseVirtualRegattaState    bool                                `json:"useVirtualRegattaState"`
	ExclusionZones            []exclusionZoneRequest              `json:"exclusionZones"`
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
	TotalMilliseconds         float64 `json:"totalMilliseconds"`
	WeatherMilliseconds       float64 `json:"weatherMilliseconds"`
	PolarMilliseconds         float64 `json:"polarMilliseconds"`
	LandMilliseconds          float64 `json:"landMilliseconds"`
	PracticalMilliseconds     float64 `json:"practicalMilliseconds"`
	IsochroneMilliseconds     float64 `json:"isochroneMilliseconds"`
	CacheHits                 int     `json:"cacheHits"`
	CacheMisses               int     `json:"cacheMisses"`
	IsochroneDurationMin      float64 `json:"isochroneDurationMinutes"`
	VirtualRegattaStateUsed   bool    `json:"virtualRegattaStateUsed"`
	VirtualRegattaGatesUsed   int     `json:"virtualRegattaGatesUsed"`
	InitialStamina            float64 `json:"initialStamina"`
	StaminaFromVirtualRegatta bool    `json:"staminaFromVirtualRegatta"`
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
	// MaxDuration borne l'horizon demandé : 2 jours en précis, 10 jours en
	// rapide (limite du GFS). Au-delà de ~7 jours la prévision devient incertaine.
	MaxDuration time.Duration
}

func calculationProfileFor(mode string) (calculationProfile, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "precise":
		return calculationProfile{IsochroneStep: 10 * time.Minute, HeadingSpacing: 6, CellSize: 0.5, MaxCandidates: 300, IncludeIsochrones: true, MaxDuration: 4 * 24 * time.Hour}, nil
	case "quick":
		return calculationProfile{IsochroneStep: 15 * time.Minute, HeadingSpacing: 30, CellSize: 1, MaxCandidates: 300, IncludeIsochrones: true, MaxDuration: 10 * 24 * time.Hour}, nil
	default:
		return calculationProfile{}, fmt.Errorf("mode de calcul %q invalide: quick ou precise attendu", mode)
	}
}

func adaptiveIsochroneDuration(distance navigation.NauticalMiles, requested time.Duration, maxSpeed polar.BoatSpeed) time.Duration {
	if requested <= 0 || maxSpeed <= 0 || distance <= 0 {
		return requested
	}
	const (
		// Fraction de la vitesse max réellement tenue vers la marque, moyenne
		// sur une vraie route (près compris). 0,6 était trop optimiste et
		// coupait la recherche avant l'arrivée ; 0,4 ne raccourcit que les
		// cibles nettement plus proches que le plafond. La boucle s'arrête de
		// toute façon dès l'arrivée, donc une estimation plus large est sûre.
		conservativeSpeedRatio = 0.4
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

// calculationDurationForRoute keeps the adaptive horizon for direct open-water
// routes. A land constraint or a mandatory gate can make the useful route much
// longer than the great-circle distance, so those cases keep the requested
// horizon as a safe upper bound.
func calculationDurationForRoute(
	ctx context.Context,
	position navigation.Position,
	destination navigation.Position,
	instant time.Time,
	distance navigation.NauticalMiles,
	requested time.Duration,
	maxSpeed polar.BoatSpeed,
	gates []routing.CourseGate,
	landConstraint routing.Constraint,
) (time.Duration, error) {
	if len(gates) > 0 {
		return requested, nil
	}
	if landConstraint == nil {
		return adaptiveIsochroneDuration(distance, requested, maxSpeed), nil
	}
	segment := routing.RouteSegment{
		From:       position,
		To:         destination,
		StartedAt:  instant,
		FinishedAt: instant.Add(requested),
	}
	allowed, err := landConstraint.Allows(ctx, segment)
	if err != nil {
		return 0, fmt.Errorf("vérification du trajet direct impossible: %w", err)
	}
	if !allowed {
		return requested, nil
	}
	return adaptiveIsochroneDuration(distance, requested, maxSpeed), nil
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
	Stamina  float64         `json:"stamina"`
	Instant  time.Time       `json:"instant"`
	Terminal bool            `json:"terminal"`
	// Les champs suivants décrivent le pas qui se termine à ce point : vent
	// au départ du pas, allure, vitesse et manœuvre éventuelle.
	WindDirection    *float64 `json:"twd,omitempty"`
	WindKnots        *float64 `json:"tws,omitempty"`
	WindAngle        *float64 `json:"twa,omitempty"`
	Tack             string   `json:"tack,omitempty"`
	SpeedKnots       *float64 `json:"speedKnots,omitempty"`
	FoilPercent      *float64 `json:"foilPercent,omitempty"`
	SpeedCoefficient *float64 `json:"speedCoefficient,omitempty"`
	Maneuver         string   `json:"maneuver,omitempty"`
}

type availableRouteStep struct {
	Position navigation.Position
	Heading  navigation.Heading
	Sail     string
	Stamina  simulation.StaminaLevel
	Instant  time.Time
	Node     routing.IsochroneNode
}

func floatPointer(value float64) *float64 { return &value }

// routePointFromNode construit le point de route atteint par un nœud, avec le
// détail du pas qui y mène.
func routePointFromNode(
	node routing.IsochroneNode,
	boatPolar *polar.Polar,
	owned equipment.EquipmentSet,
) routePointResponse {
	step := node.Candidate.Step
	point := routePointResponse{
		Position:   positionRequest{Latitude: float64(step.NextPosition.Latitude), Longitude: float64(step.NextPosition.Longitude)},
		Heading:    float64(node.Candidate.Heading),
		Sail:       step.BestSail.SailName,
		Stamina:    float64(node.State.Stamina.Level),
		Instant:    step.NextInstant,
		WindAngle:  floatPointer(float64(step.WindAngle)),
		SpeedKnots: floatPointer(float64(step.BestSail.Speed)),
	}
	point.WindDirection = floatPointer(float64(step.Wind.Direction))
	point.WindKnots = floatPointer(float64(step.WindKnots))
	switch step.Tack {
	case navigation.TackStarboard:
		point.Tack = "starboard"
	case navigation.TackPort:
		point.Tack = "port"
	}
	if boatPolar != nil && owned.Has(equipment.Foils) && boatPolar.Foil.MaxSpeedRatio > 1 {
		ratio, err := boatPolar.Foil.RatioAt(polar.WindSpeed(step.WindKnots), polar.TrueWindAngle(step.WindAngle))
		if err == nil {
			percent := float64(ratio-1) / float64(boatPolar.Foil.MaxSpeedRatio-1) * 100
			point.FoilPercent = floatPointer(math.Max(0, math.Min(100, percent)))
		}
	}
	if node.SpeedPenalty != nil && node.SpeedPenalty.AppliedPenalty != nil {
		point.SpeedCoefficient = floatPointer(float64(node.SpeedPenalty.AppliedPenalty.SpeedRatio))
	}
	if node.Maneuver != nil {
		switch {
		case node.Maneuver.Kind == simulation.ManeuverTack:
			point.Maneuver = "tack"
		case node.Maneuver.Kind == simulation.ManeuverGybe:
			point.Maneuver = "gybe"
		case node.Maneuver.SailChanged:
			point.Maneuver = "sail"
		}
	}
	return point
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
	runPracticalHeadings(response, request, nil, nil)
}

func practicalHeadingsStreamHandler(response http.ResponseWriter, request *http.Request) {
	runPracticalHeadings(response, request, &practicalHeadingsStream{}, nil)
}

func runPracticalHeadings(
	response http.ResponseWriter,
	request *http.Request,
	stream *practicalHeadingsStream,
	vrStateStore *vrStateStore,
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

	useVRPosition, useVRStamina, useGates := resolveVRSettings(payload)
	needVRState := useVRPosition || useVRStamina || useGates

	var receivedVRState *vrState
	var virtualRegattaGates []routing.CourseGate
	if needVRState {
		if vrStateStore == nil {
			writeJSONError(response, http.StatusBadRequest, "état Virtual Regatta indisponible pour ce gestionnaire")
			return
		}
		state, ok := vrStateStore.snapshot()
		if !ok {
			writeJSONError(response, http.StatusBadRequest, "aucun état Virtual Regatta reçu")
			return
		}
		receivedVRState = &state
		if useGates {
			gates, gatesErr := courseGatesFromVRState(state)
			if gatesErr != nil {
				writeJSONError(response, http.StatusBadRequest, "portes Virtual Regatta invalides: "+gatesErr.Error())
				return
			}
			virtualRegattaGates = gates
		}
		if useVRPosition {
			payload.Position = positionRequest{
				Latitude:  state.Position.Latitude,
				Longitude: state.Position.Longitude,
			}
			payload.Owned = equipmentIDsFromVRState(state)
		}
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
	if requested := time.Duration(payload.DurationMinutes * float64(time.Minute)); requested > profile.MaxDuration {
		writeJSONError(response, http.StatusBadRequest, fmt.Sprintf(
			"durée de %.0f min trop longue pour ce mode : %.0f min au plus (%.0f jours)",
			payload.DurationMinutes, profile.MaxDuration.Minutes(), profile.MaxDuration.Hours()/24,
		))
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
	// La stamina est toujours simulée : la pénalité de manœuvre — et donc
	// l'anti-zigzag — s'applique à tout calcul, que la position et la stamina
	// viennent de Virtual Regatta ou d'une saisie manuelle.
	transitionEvaluator, combineSpeedPenalties := staminaManeuverRuntime(boatPolar)
	var initialNavigation *routing.InitialNavigationState
	// Le cap et la voile courants ne sont cohérents qu'au départ réel du bateau VR.
	if useVRPosition && receivedVRState != nil {
		initialNavigation = initialNavigationFromVRState(receivedVRState)
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
	positionSourceLabel := "manuelle"
	if useVRPosition {
		positionSourceLabel = "VR"
	}
	staminaSourceLabel := "manuelle"
	if useVRStamina {
		staminaSourceLabel = "VR"
	}
	log.Printf(
		"[ROUTE] polaire=%q bateau=%q (cache=%t) · mode=%s · durée=%.0f min (%.1f j) · position=%s · stamina=%s · portes=%t · départ=%.4f,%.4f arrivée=%.4f,%.4f",
		payload.PolarPath, boatPolar.Label, polarHit, payload.Mode,
		payload.DurationMinutes, payload.DurationMinutes/1440,
		positionSourceLabel, staminaSourceLabel, useGates,
		float64(position.Latitude), float64(position.Longitude),
		float64(destination.Latitude), float64(destination.Longitude),
	)
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
	exclusionConstraint, zonesErr := exclusionConstraintFromZones(payload.ExclusionZones)
	if zonesErr != nil {
		writeJSONError(response, http.StatusBadRequest, zonesErr.Error())
		return
	}
	routeConstraints := []routing.Constraint{landConstraint}
	arrivalConstraints := []routing.Constraint{arrivalLandConstraint}
	if exclusionConstraint != nil {
		endpoints := []struct {
			label string
			point navigation.Position
		}{{"départ", position}, {"destination", destination}}
		for _, endpoint := range endpoints {
			inside, containsErr := exclusionConstraint.Contains(endpoint.point)
			if containsErr != nil {
				writeJSONError(response, http.StatusBadRequest, "vérification des zones interdites impossible pour le "+endpoint.label+": "+containsErr.Error())
				return
			}
			if inside {
				writeJSONError(response, http.StatusBadRequest, "le point de "+endpoint.label+" se trouve dans une zone interdite")
				return
			}
		}
		routeConstraints = append(routeConstraints, exclusionConstraint)
		arrivalConstraints = append(arrivalConstraints, exclusionConstraint)
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
	result.Performance.VirtualRegattaStateUsed = receivedVRState != nil
	result.Performance.VirtualRegattaGatesUsed = len(virtualRegattaGates)
	initialStamina, staminaFromVR := resolveInitialStamina(useVRStamina, receivedVRState, payload.ManualStamina)
	result.Performance.InitialStamina = float64(initialStamina.Level)
	result.Performance.StaminaFromVirtualRegatta = staminaFromVR
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
	calculationDuration, err := calculationDurationForRoute(
		calculationContext,
		position,
		destination,
		instant,
		distance,
		requestedDuration,
		boatPolar.MaxSpeed,
		virtualRegattaGates,
		allConstraints(routeConstraints...),
	)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "optimisation de l'horizon impossible: "+err.Error())
		return
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
		frontierResponses := make(map[int]isochroneFrontierResponse, layerCount)
		if stream != nil {
			progressReporter = func(progress routing.IsochroneProgress) {
				frontier, frontierErr := isochroneFrontierResponseFor(position, progress.Frontier)
				if frontierErr != nil {
					streamCallbackError = frontierErr
					cancelCalculation()
					return
				}
				frontierResponses[progress.Frontier.Index] = frontier
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
		recoveryRules := staminaRecoveryRules()
		search, err := buildIsochroneSearch(calculationContext, weatherProvider, boatPolar, routing.IsochroneSearchInput{
			Position: position, Destination: destination, Instant: instant, StepDuration: isochroneStep, LayerCount: layerCount,
			Owned: owned, HeadingSpacing: profile.HeadingSpacing, RefineHeadings: true, CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
			ArrivalRadius: navigation.NauticalMiles(1), InitialStamina: initialStamina,
			InitialNavigation:     initialNavigation,
			TransitionEvaluator:   transitionEvaluator,
			CombineSpeedPenalties: combineSpeedPenalties,
			StaminaRecoveryRules:  &recoveryRules,
			Gates:                 virtualRegattaGates,
			Constraints:           routeConstraints,
			ArrivalConstraints:    arrivalConstraints,
			Progress:              progressReporter,
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
			layer, exists := frontierResponses[frontier.Index]
			if !exists {
				layer, err = isochroneFrontierResponseFor(position, frontier)
				if err != nil {
					if stream != nil {
						stream.fail(response, "contour d’isochrone impossible: "+err.Error(), err)
					} else {
						writeJSONError(response, http.StatusBadRequest, "contour d’isochrone impossible: "+err.Error())
					}
					return
				}
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
			result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(search.Origin.Latitude), Longitude: float64(search.Origin.Longitude)}, Stamina: float64(initialStamina.Level), Instant: search.StartedAt})
			for _, routeStep := range availableSteps {
				result.Route = append(result.Route, routePointFromNode(routeStep.Node, boatPolar, owned))
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
			result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(bestRoute.Origin.Latitude), Longitude: float64(bestRoute.Origin.Longitude)}, Stamina: float64(initialStamina.Level), Instant: bestRoute.StartedAt})
			for _, routeStep := range routeSteps {
				result.Route = append(result.Route, routePointFromNode(routeStep.Node, boatPolar, owned))
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
				terminalStamina := initialStamina.Level
				if len(routeSteps) > 0 {
					terminalStamina = routeSteps[len(routeSteps)-1].Node.State.Stamina.Level
				}
				result.Route = append(result.Route, routePointResponse{Position: positionRequest{Latitude: float64(destination.Latitude), Longitude: float64(destination.Longitude)}, Stamina: float64(terminalStamina), Instant: lastInstant.Add(profile.IsochroneStep), Terminal: true})
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
			Stamina:  node.State.Stamina.Level,
			Instant:  node.Candidate.Step.NextInstant,
			Node:     node,
		}
		nodeIndex = node.ParentIndex
	}
	return steps, nil
}

func resolveVRSettings(payload practicalHeadingsRequest) (useVRPosition, useVRStamina, useGates bool) {
	// Compatibilité : un ancien client n'envoyant que useVirtualRegattaState
	// active les trois réglages sur Virtual Regatta.
	if payload.UseVirtualRegattaState {
		return true, true, true
	}
	useVRPosition = strings.EqualFold(strings.TrimSpace(payload.PositionSource), "vr")
	useVRStamina = strings.EqualFold(strings.TrimSpace(payload.StaminaSource), "vr")
	useGates = payload.UseCourseGates
	return useVRPosition, useVRStamina, useGates
}

// resolveInitialStamina choisit le niveau de stamina initial : la valeur reçue
// de Virtual Regatta lorsqu'elle est demandée et disponible, sinon la valeur
// saisie (bornée à 0..100), sinon 100 par défaut. Le second retour indique si
// la valeur provient de Virtual Regatta.
func resolveInitialStamina(useVR bool, state *vrState, manual *float64) (simulation.StaminaState, bool) {
	if useVR && state != nil && state.Stamina != nil {
		return simulation.StaminaState{Level: simulation.StaminaLevel(*state.Stamina)}, true
	}
	if manual != nil {
		level := *manual
		if level < 0 {
			level = 0
		}
		if level > 100 {
			level = 100
		}
		return simulation.StaminaState{Level: simulation.StaminaLevel(level)}, false
	}
	return simulation.StaminaState{Level: 100}, false
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






package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

// Recherche de la meilleure heure de départ d'un record : le joueur choisit
// quand il prend le départ, entre l'ouverture et la fermeture du leg. Chaque
// heure candidate est routée pendant la même durée, puis les positions
// atteintes sont comparées. Un record dure souvent plus longtemps que les
// prévisions (41 jours pour le Trophée Jules Verne, 16 jours de GFS) : on
// compare donc l'avance prise sur une durée fixe, pas un temps d'arrivée.

const (
	defaultComparisonDuration = 72 * time.Hour
	maxComparisonDuration     = 10 * 24 * time.Hour
	defaultDepartureStep      = 6 * time.Hour
	minDepartureStep          = time.Hour
	maxDepartureCount         = 40
)

// departureComparisonProfile est plus grossier que le mode rapide : il sert à
// comparer des départs entre eux, pas à suivre la route. Sur la capture du
// Trophée Jules Verne (72 h, Ultim), il classe les départs dans le même ordre
// qu'un pas de 30 min et des cellules de 3 NM, en 10 s par départ au lieu de 27.
var departureComparisonProfile = calculationProfile{
	IsochroneStep:  60 * time.Minute,
	HeadingSpacing: 10,
	CellSize:       5,
	MaxCandidates:  400,
}

type departureWindowRequest struct {
	WeatherFiles    []string               `json:"weatherFiles"`
	PolarPath       string                 `json:"polarPath"`
	Destination     positionRequest        `json:"destination"`
	Owned           []string               `json:"owned"`
	ManualStamina   *float64               `json:"manualStamina"`
	ExclusionZones  []exclusionZoneRequest `json:"exclusionZones"`
	CoastalMarginNM *float64               `json:"coastalMarginNM"`
	ArrivalRadiusNM *float64               `json:"arrivalRadiusNM"`
	// ComparisonHours est la durée de route comparée pour chaque départ
	// (72 h par défaut), StepHours l'écart entre deux départs (6 h).
	ComparisonHours float64 `json:"comparisonHours"`
	StepHours       float64 `json:"stepHours"`
}

type departureWindowStartEvent struct {
	Departures      []time.Time     `json:"departures"`
	ComparisonHours float64         `json:"comparisonHours"`
	TotalGates      int             `json:"totalGates"`
	Position        positionRequest `json:"position"`
	CoveredUntil    time.Time       `json:"coveredUntil"`
	WindowClose     *time.Time      `json:"windowClose,omitempty"`
}

// departureOutcome résume où en est la route d'un départ à la fin de la durée
// comparée (ou à l'arrivée).
type departureOutcome struct {
	Index        int             `json:"index"`
	Departure    time.Time       `json:"departure"`
	Reached      bool            `json:"reached"`
	Arrival      *time.Time      `json:"arrival,omitempty"`
	ElapsedHours float64         `json:"elapsedHours"`
	GatesCrossed int             `json:"gatesCrossed"`
	NextTarget   string          `json:"nextTarget"`
	DistanceNM   float64         `json:"distanceNM"`
	Position     positionRequest `json:"position"`
}

type departureWindowCompleteEvent struct {
	Outcomes []departureOutcome `json:"outcomes"`
	// Best est l'indice du meilleur départ dans Outcomes, -1 sans résultat.
	Best int `json:"best"`
}

// departureBetter dit si a est un meilleur départ que b : arrivé avant, sinon
// plus de portes franchies, sinon plus près de la porte suivante. La distance
// à la porte suivante ne se compare qu'à nombre de portes égal ; le centre
// des portes « none », des lignes de plusieurs milliers de milles, ne veut
// rien dire, d'où ce classement en trois temps plutôt qu'une distance totale.
func departureBetter(a, b departureOutcome) bool {
	if a.Reached != b.Reached {
		return a.Reached
	}
	if a.Reached {
		if a.ElapsedHours != b.ElapsedHours {
			return a.ElapsedHours < b.ElapsedHours
		}
		return a.Departure.Before(b.Departure)
	}
	if a.GatesCrossed != b.GatesCrossed {
		return a.GatesCrossed > b.GatesCrossed
	}
	if a.DistanceNM != b.DistanceNM {
		return a.DistanceNM < b.DistanceNM
	}
	return a.Departure.Before(b.Departure)
}

func bestDeparture(outcomes []departureOutcome) int {
	best := -1
	for index, outcome := range outcomes {
		if best < 0 || departureBetter(outcome, outcomes[best]) {
			best = index
		}
	}
	return best
}

// departureInstants liste les départs à comparer : de from (arrondi au pas
// suivant, en UTC) jusqu'à until compris, au plus maxDepartureCount.
func departureInstants(from, until time.Time, step time.Duration) []time.Time {
	if step <= 0 || until.Before(from) {
		return nil
	}
	first := from.UTC().Truncate(step)
	if first.Before(from) {
		first = first.Add(step)
	}
	instants := make([]time.Time, 0)
	for instant := first; !instant.After(until) && len(instants) < maxDepartureCount; instant = instant.Add(step) {
		instants = append(instants, instant)
	}
	return instants
}

// departureWindowBounds calcule la plage des départs possibles : pas avant
// maintenant ni avant l'ouverture du leg, pas après sa fermeture, et assez tôt
// pour que la durée comparée tienne dans les prévisions.
func departureWindowBounds(now time.Time, window *vrStateStartWindow, coveredUntil time.Time, comparison time.Duration) (time.Time, time.Time, error) {
	from := now
	until := coveredUntil.Add(-comparison)
	if window != nil && window.Open != nil {
		if open := time.UnixMilli(*window.Open); open.After(from) {
			from = open
		}
	}
	if window != nil && window.Close != nil {
		if closing := time.UnixMilli(*window.Close); closing.Before(until) {
			until = closing
		}
	}
	if until.Before(from) {
		if window != nil && window.Close != nil && !time.UnixMilli(*window.Close).After(now) {
			return from, until, errors.New("le leg est fermé : plus aucun départ possible")
		}
		return from, until, fmt.Errorf(
			"les prévisions s'arrêtent le %s UTC : trop tôt pour comparer %.0f h de route, réduisez la durée comparée",
			coveredUntil.UTC().Format("02/01 15:04"), comparison.Hours(),
		)
	}
	return from, until, nil
}

// gribCoverageEnd est la dernière échéance des fichiers météo.
func gribCoverageEnd(files []string) (time.Time, bool) {
	var last time.Time
	found := false
	for _, path := range files {
		if _, valid, ok := gribFileValidity(path); ok && (!found || valid.After(last)) {
			last, found = valid, true
		}
	}
	return last, found
}

// departureOutcomeFromSearch retient, dans la dernière frontière, le nœud le
// plus avancé sur le parcours, selon le même classement que departureBetter.
func departureOutcomeFromSearch(
	search routing.IsochroneSearch,
	gates []routing.CourseGate,
	destination navigation.Position,
) (departureOutcome, error) {
	outcome := departureOutcome{Departure: search.StartedAt}
	if len(search.Frontiers) == 0 {
		return outcome, errors.New("aucune frontière calculée")
	}
	last := search.Frontiers[len(search.Frontiers)-1]
	outcome.ElapsedHours = last.Instant.Sub(search.StartedAt).Hours()
	if search.Reached && len(last.Nodes) > 0 {
		// La recherche place le nœud arrivé en tête de la dernière frontière.
		arrival := last.Instant
		outcome.Reached = true
		outcome.Arrival = &arrival
		outcome.GatesCrossed = len(gates)
		outcome.NextTarget = "arrivée"
		position := last.Nodes[0].State.Position
		outcome.Position = positionRequest{Latitude: float64(position.Latitude), Longitude: float64(position.Longitude)}
		return outcome, nil
	}
	found := false
	for _, node := range last.Nodes {
		candidate := departureOutcome{Departure: search.StartedAt, GatesCrossed: node.CourseProgress.NextGateIndex}
		position := node.State.Position
		target := destination
		candidate.NextTarget = "arrivée"
		if index := node.CourseProgress.NextGateIndex; index < len(gates) {
			closest, err := gates[index].ClosestPoint(position)
			if err != nil {
				return outcome, err
			}
			target = closest
			candidate.NextTarget = gates[index].Name
		}
		distance, err := navigation.DistanceBetween(position, target)
		if err != nil {
			return outcome, err
		}
		candidate.DistanceNM = float64(distance)
		if !found || departureBetter(candidate, outcome) {
			candidate.ElapsedHours = outcome.ElapsedHours
			candidate.Position = positionRequest{Latitude: float64(position.Latitude), Longitude: float64(position.Longitude)}
			outcome, found = candidate, true
		}
	}
	if !found {
		return outcome, errors.New("dernière frontière vide")
	}
	return outcome, nil
}

// runDepartureWindow compare les heures de départ possibles d'un record et
// transmet un résultat par départ (flux SSE), dans l'ordre chronologique pour
// réutiliser les échéances météo déjà chargées.
func runDepartureWindow(response http.ResponseWriter, request *http.Request, vrStateStore *vrStateStore) {
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	stream := &practicalHeadingsStream{}
	var payload departureWindowRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(response, http.StatusBadRequest, "requête JSON invalide: "+err.Error())
		return
	}
	state, ok := vrStateStore.snapshot()
	if !ok {
		writeJSONError(response, http.StatusBadRequest, "aucun état Virtual Regatta reçu")
		return
	}
	if !state.PreStart {
		writeJSONError(response, http.StatusBadRequest, "le choix de l'heure de départ ne concerne qu'un record dont le départ n'est pas encore pris")
		return
	}
	if len(payload.WeatherFiles) == 0 || strings.TrimSpace(payload.PolarPath) == "" {
		writeJSONError(response, http.StatusBadRequest, "weatherFiles et polarPath sont requis")
		return
	}
	comparison := defaultComparisonDuration
	if payload.ComparisonHours > 0 {
		comparison = time.Duration(payload.ComparisonHours * float64(time.Hour))
	}
	if comparison > maxComparisonDuration {
		writeJSONError(response, http.StatusBadRequest, fmt.Sprintf("durée comparée trop longue : %.0f h au plus", maxComparisonDuration.Hours()))
		return
	}
	step := defaultDepartureStep
	if payload.StepHours > 0 {
		step = time.Duration(payload.StepHours * float64(time.Hour))
	}
	if step < minDepartureStep {
		writeJSONError(response, http.StatusBadRequest, "écart entre deux départs trop court : 1 h au moins")
		return
	}
	gates, err := courseGatesFromVRState(state)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "portes Virtual Regatta invalides: "+err.Error())
		return
	}
	owned, err := equipment.ParseSet(strings.Join(ownedEquipmentFromVRState(state, payload.Owned), ","))
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "équipements invalides: "+err.Error())
		return
	}
	position, err := parsePositionRequest(positionRequest{Latitude: state.Position.Latitude, Longitude: state.Position.Longitude})
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "ligne de départ invalide: "+err.Error())
		return
	}
	destination, err := parsePositionRequest(payload.Destination)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "destination invalide: "+err.Error())
		return
	}

	weatherFiles, _ := blendLatestGRIB(payload.WeatherFiles, gribWatcher.Status())
	coveredUntil, ok := gribCoverageEnd(weatherFiles)
	if !ok {
		writeJSONError(response, http.StatusBadRequest, "échéances GRIB illisibles")
		return
	}
	from, until, err := departureWindowBounds(time.Now(), state.StartWindow, coveredUntil, comparison)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	departures := departureInstants(from, until, step)
	if len(departures) == 0 {
		writeJSONError(response, http.StatusBadRequest, "aucune heure de départ à comparer dans la plage disponible")
		return
	}

	boatPolar, _, err := loadPolarForRequest(payload.PolarPath)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "chargement polaire impossible: "+err.Error())
		return
	}
	routeConstraints, arrivalConstraints, err := departureConstraints(payload, position, destination)
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, err.Error())
		return
	}
	arrivalRadius := navigation.NauticalMiles(1)
	if payload.ArrivalRadiusNM != nil && *payload.ArrivalRadiusNM > 0 && *payload.ArrivalRadiusNM <= 20 {
		arrivalRadius = navigation.NauticalMiles(*payload.ArrivalRadiusNM)
	}
	// Avant le départ, le jeu ne donne pas la stamina : la saisie du
	// formulaire, sinon 100 %.
	initialStamina, _ := resolveInitialStamina(false, nil, payload.ManualStamina)
	transitionEvaluator, combineSpeedPenalties := staminaManeuverRuntime(boatPolar)
	recoveryRules := staminaRecoveryRules()

	start := departureWindowStartEvent{
		Departures:      departures,
		ComparisonHours: comparison.Hours(),
		TotalGates:      len(gates),
		Position:        positionRequest{Latitude: float64(position.Latitude), Longitude: float64(position.Longitude)},
		CoveredUntil:    coveredUntil,
	}
	if state.StartWindow != nil && state.StartWindow.Close != nil {
		closing := time.UnixMilli(*state.StartWindow.Close).UTC()
		start.WindowClose = &closing
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	stream.started = true
	if err := stream.event(response, "start", start); err != nil {
		return
	}
	log.Printf("[DÉPART] %d départ(s) du %s au %s · %.0f h comparées · %d porte(s)",
		len(departures), departures[0].Format(time.RFC3339), departures[len(departures)-1].Format(time.RFC3339), comparison.Hours(), len(gates))

	profile := departureComparisonProfile
	layerCount := int(comparison / profile.IsochroneStep)
	outcomes := make([]departureOutcome, 0, len(departures))
	for index, departure := range departures {
		started := time.Now()
		files := gribFilesForWindow(weatherFiles, departure, comparison+profile.IsochroneStep)
		provider, _, err := loadWeatherProviderForRequest(files...)
		if err != nil {
			stream.fail(response, "chargement météo impossible: "+err.Error(), err)
			return
		}
		search, err := buildIsochroneSearch(ctx, provider, boatPolar, routing.IsochroneSearchInput{
			Position: position, Destination: destination, Instant: departure,
			StepDuration: profile.IsochroneStep, LayerCount: layerCount, Owned: owned,
			HeadingSpacing: profile.HeadingSpacing, CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
			ArrivalRadius: arrivalRadius, InitialStamina: initialStamina,
			TransitionEvaluator:   transitionEvaluator,
			CombineSpeedPenalties: combineSpeedPenalties,
			StaminaRecoveryRules:  &recoveryRules,
			Gates:                 gates,
			Constraints:           routeConstraints,
			ArrivalConstraints:    arrivalConstraints,
		})
		if err != nil {
			stream.fail(response, fmt.Sprintf("calcul du départ du %s impossible: %v", departure.Format("02/01 15:04"), err), err)
			return
		}
		outcome, err := departureOutcomeFromSearch(search, gates, destination)
		if err != nil {
			stream.fail(response, fmt.Sprintf("résultat du départ du %s illisible: %v", departure.Format("02/01 15:04"), err), err)
			return
		}
		outcome.Index = index
		outcomes = append(outcomes, outcome)
		log.Printf("[DÉPART] %s : %d porte(s), %.0f NM de %q (%.1f s)",
			departure.Format(time.RFC3339), outcome.GatesCrossed, outcome.DistanceNM, outcome.NextTarget, time.Since(started).Seconds())
		if err := stream.event(response, "departure", outcome); err != nil {
			return
		}
	}
	_ = stream.event(response, "complete", departureWindowCompleteEvent{Outcomes: outcomes, Best: bestDeparture(outcomes)})
}

// departureConstraints reprend les contraintes du calcul de route : terres
// avec la marge côtière, zones interdites.
func departureConstraints(payload departureWindowRequest, position, destination navigation.Position) ([]routing.Constraint, []routing.Constraint, error) {
	landConstraint, _, err := loadLandForRequest(landGeoJSONPath)
	if err != nil {
		return nil, nil, fmt.Errorf("données terrestres indisponibles: %w", err)
	}
	arrivalLand := landConstraint
	margin := defaultCoastalMarginNM
	if payload.CoastalMarginNM != nil {
		margin = *payload.CoastalMarginNM
	}
	if !(margin >= 0 && margin <= 10) {
		return nil, nil, errors.New("marge côtière invalide: valeur entre 0 et 10 NM attendue")
	}
	landConstraint, err = landConstraint.WithCoastalMargin(navigation.NauticalMiles(margin))
	if err != nil {
		return nil, nil, fmt.Errorf("marge côtière impossible: %w", err)
	}
	routeConstraints := []routing.Constraint{landConstraint}
	arrivalConstraints := []routing.Constraint{arrivalLand}
	zones, err := exclusionConstraintFromZones(payload.ExclusionZones)
	if err != nil {
		return nil, nil, err
	}
	if zones != nil {
		for label, point := range map[string]navigation.Position{"départ": position, "destination": destination} {
			inside, err := zones.Contains(point)
			if err != nil {
				return nil, nil, fmt.Errorf("vérification des zones interdites impossible pour le %s: %w", label, err)
			}
			if inside {
				return nil, nil, fmt.Errorf("le point de %s se trouve dans une zone interdite", label)
			}
		}
		routeConstraints = append(routeConstraints, zones)
		arrivalConstraints = append(arrivalConstraints, zones)
	}
	return routeConstraints, arrivalConstraints, nil
}

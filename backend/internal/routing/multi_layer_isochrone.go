package routing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// IsochroneSearchInput describes a multi-layer isochrone exploration.
type IsochroneSearchInput struct {
	Position       navigation.Position
	Destination    navigation.Position
	Instant        time.Time
	StepDuration   time.Duration
	LayerCount     int
	Owned          equipment.EquipmentSet
	HeadingSpacing float64
	// RefineHeadings ajoute des caps entiers autour des meilleurs caps de la
	// grille régulière, pour que la route ne dépende pas du pas de la grille.
	RefineHeadings        bool
	CellSize              navigation.NauticalMiles
	MaxCandidates         int
	ArrivalRadius         navigation.NauticalMiles
	InitialStamina        simulation.StaminaState
	InitialNavigation     *InitialNavigationState
	TransitionEvaluator   *simulation.TransitionEvaluator
	CombineSpeedPenalties simulation.ManeuverSpeedPenaltyCombiner
	StaminaRecoveryRules  *simulation.StaminaRecoveryRules
	Gates                 []CourseGate
	Constraints           []Constraint
	// ArrivalConstraints are applied only to the final segment to the
	// destination. When nil, Constraints are used for backward compatibility.
	ArrivalConstraints []Constraint
	Progress           IsochroneProgressReporter
}

// IsochroneNode represents one retained position and its parent in the
// preceding frontier. ParentIndex is -1 for the first frontier.
type IsochroneNode struct {
	ParentIndex             int
	Candidate               HeadingCandidate
	InitialDistance         navigation.NauticalMiles
	RemainingDistance       navigation.NauticalMiles
	Progress                DistanceProgress
	CourseTarget            navigation.Position
	TargetInitialDistance   navigation.NauticalMiles
	TargetRemainingDistance navigation.NauticalMiles
	TargetProgress          DistanceProgress
	Maneuver                *simulation.ManeuverState
	Transition              *simulation.TransitionEvaluation
	SpeedPenalty            *simulation.SpeedPenaltyApplication
	CourseProgress          CourseProgress
	State                   simulation.BoatState
}

type isochroneParentState struct {
	Position       navigation.Position
	Stamina        simulation.StaminaState
	BoatState      simulation.BoatState
	WindAngle      navigation.TrueWindAngle
	CourseProgress CourseProgress
	HasBoatState   bool
}

// IsochroneFrontier contains the retained positions reached at one instant.
type IsochroneFrontier struct {
	Index    int
	Instant  time.Time
	CellSize navigation.NauticalMiles
	Nodes    []IsochroneNode
}

// IsochroneSearch contains every retained frontier of an exploration.
type IsochroneSearch struct {
	Origin         navigation.Position
	Destination    navigation.Position
	StartedAt      time.Time
	StepDuration   time.Duration
	ArrivalRadius  navigation.NauticalMiles
	InitialStamina simulation.StaminaState
	InitialState   *simulation.BoatState
	Gates          []CourseGate
	MaxCandidates  int
	Reached        bool
	Frontiers      []IsochroneFrontier
}

// Best returns the best retained node from the final frontier.
func (search IsochroneSearch) Best() (IsochroneNode, bool) {
	if len(search.Frontiers) == 0 {
		return IsochroneNode{}, false
	}
	final := search.Frontiers[len(search.Frontiers)-1]
	if len(final.Nodes) == 0 {
		return IsochroneNode{}, false
	}
	return final.Nodes[0], true
}

// BuildIsochroneSearch expands every retained node at each step and applies
// spatial pruning before building the following frontier.
func BuildIsochroneSearch(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input IsochroneSearchInput,
) (IsochroneSearch, error) {
	if ctx == nil {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: contexte requis",
		)
	}
	if err := ctx.Err(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones annulée avant le calcul: %w",
			err,
		)
	}
	if weatherProvider == nil {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: fournisseur météo requis",
		)
	}
	if boatPolar == nil {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: polaire requise",
		)
	}
	if err := input.Position.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: position initiale invalide: %w",
			err,
		)
	}
	if err := input.Destination.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: destination invalide: %w",
			err,
		)
	}
	if input.Instant.IsZero() {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: instant initial requis",
		)
	}
	if input.StepDuration <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: durée de pas strictement positive requise",
		)
	}
	if input.LayerCount <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: nombre de couches strictement positif requis",
		)
	}
	if err := input.Owned.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: équipements invalides: %w",
			err,
		)
	}
	preparedEffects, err := equipment.ResolveEffects(boatPolar, input.Owned)
	if err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: résolution des équipements impossible: %w",
			err,
		)
	}
	if err := input.InitialStamina.Validate(); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: stamina initiale invalide: %w",
			err,
		)
	}
	if input.StaminaRecoveryRules != nil {
		if err := input.StaminaRecoveryRules.Validate(); err != nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction des isochrones impossible: récupération de stamina invalide: %w",
				err,
			)
		}
	}
	initialCourseProgress := CourseProgress{}
	if err := initialCourseProgress.Validate(input.Gates); err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: parcours invalide: %w",
			err,
		)
	}
	for index, constraint := range input.Constraints {
		if constraint == nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction des isochrones impossible: contrainte %d absente",
				index+1,
			)
		}
	}
	for index, constraint := range input.ArrivalConstraints {
		if constraint == nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction des isochrones impossible: contrainte d'arrivée %d absente",
				index+1,
			)
		}
	}

	headings, err := GenerateHeadingGrid(input.HeadingSpacing)
	if err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: grille de caps invalide: %w",
			err,
		)
	}

	cellSize := float64(input.CellSize)
	if math.IsNaN(cellSize) ||
		math.IsInf(cellSize, 0) ||
		input.CellSize <= 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: taille de cellule finie strictement positive requise",
		)
	}
	if input.MaxCandidates != 0 && input.MaxCandidates < 4 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: la limite doit être nulle ou au moins égale à 4 candidats",
		)
	}
	arrivalRadius := float64(input.ArrivalRadius)
	if math.IsNaN(arrivalRadius) ||
		math.IsInf(arrivalRadius, 0) ||
		input.ArrivalRadius < 0 {
		return IsochroneSearch{}, errors.New(
			"routing: construction des isochrones impossible: rayon d'arrivée fini positif ou nul requis",
		)
	}

	initialDistance, err := navigation.DistanceBetween(
		input.Position,
		input.Destination,
	)
	if err != nil {
		return IsochroneSearch{}, fmt.Errorf(
			"routing: construction des isochrones impossible: distance initiale impossible: %w",
			err,
		)
	}

	courseWaypoints := make([]courseWaypoint, len(input.Gates)+1)
	targetInitialDistances := make(
		[]navigation.NauticalMiles,
		len(courseWaypoints),
	)
	for nextGateIndex := range courseWaypoints {
		waypoint := courseWaypoint{destination: input.Destination}
		if nextGateIndex < len(input.Gates) {
			waypoint = courseWaypoint{gate: &input.Gates[nextGateIndex]}
		}
		targetInitialDistance, _, distanceErr := waypoint.distanceFrom(
			input.Position,
		)
		if distanceErr != nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction des isochrones impossible: distance initiale vers la cible de parcours %d impossible: %w",
				nextGateIndex+1,
				distanceErr,
			)
		}
		courseWaypoints[nextGateIndex] = waypoint
		targetInitialDistances[nextGateIndex] = targetInitialDistance
	}

	initialParent := isochroneParentState{
		Position:       input.Position,
		Stamina:        input.InitialStamina,
		CourseProgress: initialCourseProgress,
	}
	var initialState *simulation.BoatState
	if input.InitialNavigation != nil {
		resolvedState, windAngle, resolveErr := resolveInitialBoatState(
			ctx,
			weatherProvider,
			boatPolar,
			input.Position,
			input.Instant,
			input.InitialStamina,
			input.Owned,
			*input.InitialNavigation,
		)
		if resolveErr != nil {
			return IsochroneSearch{}, fmt.Errorf(
				"routing: construction des isochrones impossible: %w",
				resolveErr,
			)
		}
		initialParent.BoatState = resolvedState
		initialParent.WindAngle = windAngle
		initialParent.HasBoatState = true
		initialState = &resolvedState
	}

	search := IsochroneSearch{
		Origin:         input.Position,
		Destination:    input.Destination,
		StartedAt:      input.Instant,
		StepDuration:   input.StepDuration,
		ArrivalRadius:  input.ArrivalRadius,
		InitialStamina: input.InitialStamina,
		InitialState:   initialState,
		Gates:          append([]CourseGate(nil), input.Gates...),
		MaxCandidates:  input.MaxCandidates,
		Frontiers:      make([]IsochroneFrontier, 0, input.LayerCount),
	}
	arrivalConstraints := input.ArrivalConstraints
	if arrivalConstraints == nil {
		arrivalConstraints = input.Constraints
	}
	parentStates := []isochroneParentState{initialParent}
	currentInstant := input.Instant

	for layerIndex := 0; layerIndex < input.LayerCount; layerIndex++ {
		if err := ctx.Err(); err != nil {
			return search, fmt.Errorf(
				"routing: construction interrompue avant la couche %d: %w",
				layerIndex+1,
				err,
			)
		}

		expanded := make(
			[]IsochroneNode,
			0,
			len(parentStates)*len(headings),
		)

		var refinement *headingRefinement
		if input.RefineHeadings {
			refinement = &headingRefinement{spacing: input.HeadingSpacing, waypoints: courseWaypoints}
		}
		parentCandidates, err := evaluateParentHeadingCandidates(
			ctx,
			weatherProvider,
			boatPolar,
			parentStates,
			currentInstant,
			input.StepDuration,
			input.Owned,
			&preparedEffects,
			headings,
			refinement,
		)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return search, fmt.Errorf(
					"routing: construction interrompue à la couche %d: %w",
					layerIndex+1,
					contextErr,
				)
			}
			return search, fmt.Errorf(
				"routing: construction de la couche %d impossible: développement d'un parent impossible: %w",
				layerIndex+1,
				err,
			)
		}

		for parentIndex, parentState := range parentStates {
			candidates := parentCandidates[parentIndex]

			lineageParent := parentIndex
			if layerIndex == 0 {
				lineageParent = -1
			}

			for _, candidate := range candidates {
				maneuver, transition, speedPenalty, err :=
					evaluateCandidateTransitionAndPenalty(
						parentState,
						&candidate,
						input.Owned,
						input.TransitionEvaluator,
						input.CombineSpeedPenalties,
					)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: candidat au cap %.6f° invalide: %w",
						layerIndex+1,
						candidate.Heading,
						err,
					)
				}
				segment := RouteSegment{
					From:       parentState.Position,
					To:         candidate.Step.NextPosition,
					StartedAt:  candidate.Step.Instant,
					FinishedAt: candidate.Step.NextInstant,
				}
				if len(input.Constraints) > 0 {
					allowed, constraintErr := segmentAllowed(
						ctx,
						input.Constraints,
						segment,
					)
					if constraintErr != nil {
						return IsochroneSearch{}, fmt.Errorf(
							"routing: construction de la couche %d impossible: contrainte du candidat au cap %.6f° invalide: %w",
							layerIndex+1,
							candidate.Heading,
							constraintErr,
						)
					}
					if !allowed {
						continue
					}
				}
				nextCourseProgress, err := parentState.CourseProgress.Advance(
					input.Gates,
					segment,
				)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: progression du parcours au cap %.6f° invalide: %w",
						layerIndex+1,
						candidate.Heading,
						err,
					)
				}

				remainingDistance, err := navigation.DistanceBetween(
					candidate.Step.NextPosition,
					input.Destination,
				)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: distance restante impossible: %w",
						layerIndex+1,
						err,
					)
				}

				targetInitialDistance :=
					targetInitialDistances[nextCourseProgress.NextGateIndex]
				targetRemainingDistance, courseTarget, err := courseWaypoints[nextCourseProgress.NextGateIndex].distanceFrom(
					candidate.Step.NextPosition,
				)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: distance vers la cible de parcours impossible: %w",
						layerIndex+1,
						err,
					)
				}

				nextStamina, err := nextStaminaForStep(
					parentState.Stamina,
					maneuver,
					transition,
					input.StepDuration,
					float64(candidate.Step.WindKnots),
					input.StaminaRecoveryRules,
				)
				if err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: stamina du candidat au cap %.6f° invalide: %w",
						layerIndex+1,
						candidate.Heading,
						err,
					)
				}
				var activeSpeedPenalty *polar.ManeuverPenalty
				if speedPenalty != nil {
					activeSpeedPenalty = speedPenalty.RemainingPenalty
				}
				state := simulation.BoatState{
					Position:           candidate.Step.NextPosition,
					Instant:            candidate.Step.NextInstant,
					Heading:            candidate.Step.Heading,
					SailID:             candidate.Step.BestSail.SailID,
					Tack:               candidate.Step.Tack,
					Stamina:            nextStamina,
					ActiveSpeedPenalty: activeSpeedPenalty,
				}
				if err := state.Validate(); err != nil {
					return IsochroneSearch{}, fmt.Errorf(
						"routing: construction de la couche %d impossible: état atteint invalide: %w",
						layerIndex+1,
						err,
					)
				}

				expanded = append(expanded, IsochroneNode{
					ParentIndex:       lineageParent,
					Candidate:         candidate,
					InitialDistance:   initialDistance,
					RemainingDistance: remainingDistance,
					Progress: DistanceProgress(
						float64(initialDistance - remainingDistance),
					),
					CourseTarget:            courseTarget,
					TargetInitialDistance:   targetInitialDistance,
					TargetRemainingDistance: targetRemainingDistance,
					TargetProgress: DistanceProgress(
						float64(targetInitialDistance - targetRemainingDistance),
					),
					Maneuver:       maneuver,
					Transition:     transition,
					SpeedPenalty:   speedPenalty,
					CourseProgress: nextCourseProgress,
					State:          state,
				})
			}
		}

		if len(expanded) == 0 {
			return search, fmt.Errorf(
				"routing: aucune trajectoire autorisée à la couche %d",
				layerIndex+1,
			)
		}
		nodes, effectiveCellSize, err := pruneIsochroneNodesToLimit(
			input.Position,
			expanded,
			cellSize,
			input.MaxCandidates,
		)
		if err != nil {
			return search, fmt.Errorf(
				"routing: construction de la couche %d impossible: élagage spatial impossible: %w",
				layerIndex+1,
				err,
			)
		}

		nextInstant := currentInstant.Add(input.StepDuration)
		search.Reached = false
		for index := range nodes {
			complete, completeErr := nodes[index].CourseProgress.Complete(
				input.Gates,
			)
			if completeErr != nil {
				return search, fmt.Errorf(
					"routing: construction de la couche %d impossible: progression du parcours du candidat %d invalide: %w",
					layerIndex+1,
					index+1,
					completeErr,
				)
			}
			if complete && nodes[index].RemainingDistance <= input.ArrivalRadius {
				finalSegment := RouteSegment{
					From:       nodes[index].State.Position,
					To:         input.Destination,
					StartedAt:  nodes[index].State.Instant,
					FinishedAt: nodes[index].State.Instant.Add(input.StepDuration),
				}
				allowed, constraintErr := segmentAllowed(ctx, arrivalConstraints, finalSegment)
				if constraintErr != nil {
					return search, fmt.Errorf(
						"routing: vérification de l'approche finale à la couche %d impossible: %w",
						layerIndex+1,
						constraintErr,
					)
				}
				if !allowed {
					continue
				}
				nodes[0], nodes[index] = nodes[index], nodes[0]
				search.Reached = true
				break
			}
		}
		frontier := IsochroneFrontier{
			Index:    layerIndex + 1,
			Instant:  nextInstant,
			CellSize: navigation.NauticalMiles(effectiveCellSize),
			Nodes:    nodes,
		}
		search.Frontiers = append(search.Frontiers, frontier)
		if input.Progress != nil {
			input.Progress(IsochroneProgress{
				CompletedLayers: len(search.Frontiers),
				TotalLayers:     input.LayerCount,
				Instant:         frontier.Instant,
				CandidateCount:  len(frontier.Nodes),
				CellSize:        frontier.CellSize,
				Reached:         search.Reached,
				Frontier:        frontier,
			})
		}
		if search.Reached {
			break
		}

		parentStates = make([]isochroneParentState, len(nodes))
		for index, node := range nodes {
			parentStates[index] = isochroneParentState{
				Position:       node.State.Position,
				Stamina:        node.State.Stamina,
				BoatState:      node.State,
				WindAngle:      node.Candidate.Step.WindAngle,
				CourseProgress: node.CourseProgress,
				HasBoatState:   true,
			}
		}
		currentInstant = nextInstant
	}

	return search, nil
}

func candidateManeuver(
	parent isochroneParentState,
	candidate HeadingCandidate,
) (*simulation.ManeuverState, error) {
	if !parent.HasBoatState {
		return nil, nil
	}
	if parent.BoatState.Tack != navigation.TackUndefined &&
		candidate.Step.Tack != navigation.TackUndefined &&
		parent.BoatState.Tack != candidate.Step.Tack &&
		parent.WindAngle+candidate.Step.WindAngle == 180 {
		// With diametrically opposed headings, the endpoints alone do not
		// reveal whether the boat crossed upwind or downwind.
		return nil, nil
	}

	maneuver, err := simulation.DetectManeuver(
		simulation.ManeuverTransition{
			PreviousTack:      parent.BoatState.Tack,
			NextTack:          candidate.Step.Tack,
			PreviousWindAngle: parent.WindAngle,
			NextWindAngle:     candidate.Step.WindAngle,
			PreviousSailID:    parent.BoatState.SailID,
			NextSailID:        candidate.Step.BestSail.SailID,
		},
	)
	if err != nil {
		return nil, err
	}
	return &maneuver, nil
}

func pruneIsochroneNodesToLimit(
	origin navigation.Position,
	nodes []IsochroneNode,
	minimumCellSize float64,
	maxCandidates int,
) ([]IsochroneNode, float64, error) {
	// Élagage state-aware à la résolution de base : on retire les états
	// dominés sans fusionner les cellules (ce qui pincerait la frontière en
	// couloir).
	selected, err := pruneIsochroneNodesUnsorted(origin, nodes, minimumCellSize)
	if err != nil {
		return nil, 0, err
	}
	if maxCandidates == 0 || len(selected) <= maxCandidates {
		sortIsochroneNodes(selected)
		return selected, minimumCellSize, nil
	}
	// Trop de nœuds : on borne en répartissant par secteurs de cap depuis
	// l'origine (le meilleur par secteur). La frontière reste large et la
	// diversité latérale — donc les routes alternatives — survit.
	kept := retainAngularSectors(origin, selected, maxCandidates)
	sortIsochroneNodes(kept)
	return kept, minimumCellSize, nil
}

// nodeBearingRadians retourne le cap planaire origine -> position, corrigé
// de la compression des longitudes par la latitude. Suffisant pour le
// découpage en secteurs (seul l'ordre angulaire compte).
func nodeBearingRadians(origin navigation.Position, position navigation.Position) float64 {
	deltaLat := float64(position.Latitude) - float64(origin.Latitude)
	meanLat := (float64(position.Latitude) + float64(origin.Latitude)) / 2
	deltaLon := (float64(position.Longitude) - float64(origin.Longitude)) *
		math.Cos(meanLat*math.Pi/180)
	return math.Atan2(deltaLon, deltaLat)
}

// nodeBetter ordonne deux nœuds comme sortIsochroneNodes : porte franchie,
// puis progression vers la cible, puis distance, puis stamina.
func nodeBetter(a, b IsochroneNode) bool {
	if a.CourseProgress.NextGateIndex != b.CourseProgress.NextGateIndex {
		return a.CourseProgress.NextGateIndex > b.CourseProgress.NextGateIndex
	}
	if a.TargetProgress != b.TargetProgress {
		return a.TargetProgress > b.TargetProgress
	}
	if a.Progress != b.Progress {
		return a.Progress > b.Progress
	}
	return a.State.Stamina.Level > b.State.Stamina.Level
}

// retainAngularSectors borne le nombre de nœuds en préservant la largeur de
// la frontière : l'étendue des caps (depuis l'origine) est découpée en
// maxCandidates secteurs, et on garde le meilleur nœud de chaque secteur.
func retainAngularSectors(
	origin navigation.Position,
	nodes []IsochroneNode,
	maxCandidates int,
) []IsochroneNode {
	if maxCandidates <= 0 || len(nodes) <= maxCandidates {
		return nodes
	}
	bearings := make([]float64, len(nodes))
	for index, node := range nodes {
		bearings[index] = nodeBearingRadians(origin, node.Candidate.Step.NextPosition)
	}
	ordered := append([]float64(nil), bearings...)
	sort.Float64s(ordered)
	// Étendue robuste au passage ±180° : le plus grand trou sur le cercle est
	// hors frontière, l'étendue utile est son complément.
	largestGap, gapIndex := -1.0, 0
	for index := range ordered {
		next := ordered[0] + 2*math.Pi
		if index+1 < len(ordered) {
			next = ordered[index+1]
		}
		if gap := next - ordered[index]; gap > largestGap {
			largestGap = gap
			gapIndex = index
		}
	}
	start := ordered[(gapIndex+1)%len(ordered)]
	span := 2*math.Pi - largestGap
	if span <= 1e-9 {
		sortIsochroneNodes(nodes)
		return nodes[:maxCandidates]
	}
	width := span / float64(maxCandidates)
	best := make([]*IsochroneNode, maxCandidates)
	for index := range nodes {
		relative := bearings[index] - start
		for relative < 0 {
			relative += 2 * math.Pi
		}
		sector := int(relative / width)
		if sector < 0 {
			sector = 0
		}
		if sector >= maxCandidates {
			sector = maxCandidates - 1
		}
		if best[sector] == nil || nodeBetter(nodes[index], *best[sector]) {
			candidate := nodes[index]
			best[sector] = &candidate
		}
	}
	kept := make([]IsochroneNode, 0, maxCandidates)
	for _, node := range best {
		if node != nil {
			kept = append(kept, *node)
		}
	}
	return kept
}

const maximumStateAwareCellSize = 360 * nauticalMilesPerDegree

type isochroneStateCell struct {
	spatialCell
	heading                navigation.Heading
	sailID                 polar.SailID
	tack                   navigation.Tack
	hasActiveSpeedPenalty  bool
	activeSpeedRatio       polar.SpeedRatio
	activePenaltyRemaining time.Duration
	nextGateIndex          int
}

func pruneIsochroneNodes(
	origin navigation.Position,
	nodes []IsochroneNode,
	cellSize float64,
) ([]IsochroneNode, error) {
	selected, err := pruneIsochroneNodesUnsorted(origin, nodes, cellSize)
	if err != nil {
		return nil, err
	}
	sortIsochroneNodes(selected)
	return selected, nil
}

// pruneIsochroneNodesUnsorted performs state-aware grouping without sorting.
// The adaptive cell-size loop only needs the selected count until its final
// pass, so sorting intermediate results would duplicate substantial work.
func pruneIsochroneNodesUnsorted(
	origin navigation.Position,
	nodes []IsochroneNode,
	cellSize float64,
) ([]IsochroneNode, error) {
	if len(nodes) == 0 {
		return nil, errors.New("aucune position à conserver")
	}

	groups := make(
		map[isochroneStateCell][]IsochroneNode,
		len(nodes),
	)
	keys := make([]isochroneStateCell, 0, len(nodes))
	for index, node := range nodes {
		if err := node.State.Stamina.Validate(); err != nil {
			return nil, fmt.Errorf(
				"stamina de la position %d invalide: %w",
				index+1,
				err,
			)
		}
		cell, err := spatialCellFor(
			origin,
			node.Candidate.Step.NextPosition,
			cellSize,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cellule de la position %d impossible: %w",
				index+1,
				err,
			)
		}

		key := isochroneStateCell{
			spatialCell:   cell,
			heading:       node.State.Heading,
			sailID:        node.State.SailID,
			tack:          node.State.Tack,
			nextGateIndex: node.CourseProgress.NextGateIndex,
		}
		if node.State.ActiveSpeedPenalty != nil {
			key.hasActiveSpeedPenalty = true
			key.activeSpeedRatio =
				node.State.ActiveSpeedPenalty.SpeedRatio
			key.activePenaltyRemaining =
				node.State.ActiveSpeedPenalty.Duration
		}
		group, exists := groups[key]
		if !exists {
			keys = append(keys, key)
		}

		dominated := false
		for _, existing := range group {
			if isochroneNodeDominates(existing, node) {
				dominated = true
				break
			}
		}
		if dominated {
			continue
		}

		kept := group[:0]
		for _, existing := range group {
			if !isochroneNodeDominates(node, existing) {
				kept = append(kept, existing)
			}
		}
		groups[key] = append(kept, node)
	}

	selected := make([]IsochroneNode, 0, len(nodes))
	for _, key := range keys {
		selected = append(selected, groups[key]...)
	}
	return selected, nil
}

func sortIsochroneNodes(selected []IsochroneNode) {
	sort.SliceStable(selected, func(left, right int) bool {
		if selected[left].CourseProgress.NextGateIndex !=
			selected[right].CourseProgress.NextGateIndex {
			return selected[left].CourseProgress.NextGateIndex >
				selected[right].CourseProgress.NextGateIndex
		}
		if selected[left].TargetProgress !=
			selected[right].TargetProgress {
			return selected[left].TargetProgress >
				selected[right].TargetProgress
		}
		if selected[left].Progress != selected[right].Progress {
			return selected[left].Progress > selected[right].Progress
		}
		if selected[left].State.Stamina.Level !=
			selected[right].State.Stamina.Level {
			return selected[left].State.Stamina.Level >
				selected[right].State.Stamina.Level
		}
		if selected[left].State.Heading != selected[right].State.Heading {
			return selected[left].State.Heading <
				selected[right].State.Heading
		}
		if selected[left].State.SailID != selected[right].State.SailID {
			return selected[left].State.SailID <
				selected[right].State.SailID
		}
		return selected[left].State.Tack < selected[right].State.Tack
	})
}

func isochroneNodeDominates(
	left,
	right IsochroneNode,
) bool {
	return left.TargetProgress >= right.TargetProgress &&
		left.State.Stamina.Level >= right.State.Stamina.Level
}

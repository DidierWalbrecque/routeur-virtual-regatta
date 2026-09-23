package routing

import (
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// IsochroneProgress describes the last fully retained frontier of an ongoing
// search. Reports are emitted synchronously after spatial pruning.
type IsochroneProgress struct {
	CompletedLayers int
	TotalLayers     int
	Instant         time.Time
	CandidateCount  int
	CellSize        navigation.NauticalMiles
	Reached         bool
	// Frontier is the completed frontier associated with this report. Its
	// nodes are read-only for the reporter and allow streaming consumers to
	// serialize the contour before the search continues.
	Frontier IsochroneFrontier
}

// IsochroneProgressReporter receives one report for each completed frontier.
// It may cancel the context supplied to BuildIsochroneSearch to stop later work.
type IsochroneProgressReporter func(IsochroneProgress)

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

type vrStatePosition struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type vrStateSail struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Automatic bool   `json:"automatic,omitempty"`
}

type vrStateCheckpoint struct {
	Group   int             `json:"group"`
	ID      int             `json:"id"`
	Name    string          `json:"name"`
	Display string          `json:"display"`
	Engine  string          `json:"engine"`
	Rhumb   bool            `json:"rhumb"`
	Side    string          `json:"side"`
	Start   vrStatePosition `json:"start"`
	End     vrStatePosition `json:"end"`
	OnLand  *bool           `json:"onLand,omitempty"`
}

type vrState struct {
	RaceID            string              `json:"raceId"`
	LegNum            string              `json:"legNum"`
	UserID            string              `json:"userId"`
	DisplayName       string              `json:"displayName"`
	Position          vrStatePosition     `json:"position"`
	Stamina           *float64            `json:"stamina"`
	Options           []string            `json:"options"`
	FullOptions       bool                `json:"fullOptions"`
	Sails             []vrStateSail       `json:"sails"`
	CurrentSail       *vrStateSail        `json:"currentSail"`
	Checkpoints       []vrStateCheckpoint `json:"checkpoints"`
	GateGroupCounters []bool              `json:"gateGroupCounters"`
	Heading           *float64            `json:"heading"`
	Speed             *float64            `json:"speed"`
	State             string              `json:"state"`
	UpdatedAt         *int64              `json:"updatedAt"`
	SourceURL         string              `json:"sourceUrl,omitempty"`
	ReceivedAt        int64               `json:"receivedAt"`
}

type vrStateStore struct {
	mu      sync.RWMutex
	current *vrState
}

func newVRStateStore() *vrStateStore {
	return &vrStateStore{}
}

func (store *vrStateStore) post(response http.ResponseWriter, request *http.Request) {
	var state vrState
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	if err := decoder.Decode(&state); err != nil {
		writeJSONError(response, http.StatusBadRequest, "état Virtual Regatta invalide: "+err.Error())
		return
	}
	if err := validateVRState(state); err != nil {
		writeJSONError(response, http.StatusBadRequest, "état Virtual Regatta invalide: "+err.Error())
		return
	}
	annotateVRStateCheckpoints(&state)

	state.ReceivedAt = time.Now().UnixMilli()
	store.mu.Lock()
	store.current = &state
	store.mu.Unlock()
	writeJSON(response, http.StatusOK, state)
}

// annotateVRStateCheckpoints marks gates whose geometric midpoint is on land.
// The marks themselves may legitimately be on the coast, so filtering is based
// on the passage line's midpoint rather than either endpoint.
func annotateVRStateCheckpoints(state *vrState) {
	if state == nil || len(state.Checkpoints) == 0 || loadLandForRequest == nil {
		return
	}
	var landConstraint *land.GeoJSONConstraint
	for _, path := range []string{landGeoJSONPath, "data/land/natural-earth-10m-land.geojson"} {
		candidate, _, loadErr := loadLandForRequest(path)
		if loadErr == nil && candidate != nil {
			landConstraint = candidate
			break
		}
	}
	if landConstraint == nil {
		return
	}
	for index := range state.Checkpoints {
		checkpoint := &state.Checkpoints[index]
		midpoint := navigation.Position{
			Latitude:  navigation.Latitude((checkpoint.Start.Latitude + checkpoint.End.Latitude) / 2),
			Longitude: navigation.Longitude((checkpoint.Start.Longitude + checkpoint.End.Longitude) / 2),
		}
		onLand, containsErr := landConstraint.Contains(midpoint)
		if containsErr == nil {
			checkpoint.OnLand = &onLand
		}
	}
}

func (store *vrStateStore) get(response http.ResponseWriter, _ *http.Request) {
	state, ok := store.snapshot()
	if !ok {
		writeJSONError(response, http.StatusNotFound, "aucun état Virtual Regatta reçu")
		return
	}
	writeJSON(response, http.StatusOK, state)
}

func (store *vrStateStore) snapshot() (vrState, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.current == nil {
		return vrState{}, false
	}

	state := *store.current
	state.Options = append([]string(nil), store.current.Options...)
	state.Sails = append([]vrStateSail(nil), store.current.Sails...)
	state.Checkpoints = append([]vrStateCheckpoint(nil), store.current.Checkpoints...)
	state.GateGroupCounters = append([]bool(nil), store.current.GateGroupCounters...)
	if store.current.CurrentSail != nil {
		currentSail := *store.current.CurrentSail
		state.CurrentSail = &currentSail
	}
	return state, true
}

func courseGatesFromVRState(state vrState) ([]routing.CourseGate, error) {
	gates := make([]routing.CourseGate, 0, len(state.Checkpoints))
	for index, checkpoint := range state.Checkpoints {
		if checkpoint.Group > 0 && checkpoint.Group <= len(state.GateGroupCounters) && state.GateGroupCounters[checkpoint.Group-1] {
			continue
		}
		name := strings.TrimSpace(checkpoint.Name)
		if name == "" {
			name = fmt.Sprintf("Checkpoint %d.%d", checkpoint.Group, checkpoint.ID)
		}

		direction := routing.GateEitherDirection
		if !checkpoint.Rhumb {
			switch strings.ToLower(strings.TrimSpace(checkpoint.Side)) {
			case "stbd", "starboard":
				direction = routing.GateLeftToRight
			case "port":
				direction = routing.GateRightToLeft
			case "", "any", "both":
				// Some checkpoint types do not impose a side.
			default:
				return nil, fmt.Errorf("porte Virtual Regatta %d: côté %q invalide", index+1, checkpoint.Side)
			}
		}

		gates = append(gates, routing.CourseGate{
			Name: name,
			FirstMark: navigation.Position{
				Latitude:  navigation.Latitude(checkpoint.Start.Latitude),
				Longitude: navigation.Longitude(checkpoint.Start.Longitude),
			},
			SecondMark: navigation.Position{
				Latitude:  navigation.Latitude(checkpoint.End.Latitude),
				Longitude: navigation.Longitude(checkpoint.End.Longitude),
			},
			Direction: direction,
		})
	}
	return gates, nil
}

func initialNavigationFromVRState(state *vrState) *routing.InitialNavigationState {
	if state == nil || state.Heading == nil || state.CurrentSail == nil {
		return nil
	}
	heading := navigation.Heading(*state.Heading)
	if heading.Validate() != nil || state.CurrentSail.ID <= 0 {
		return nil
	}
	return &routing.InitialNavigationState{
		Heading: heading,
		SailID:  polar.SailID(state.CurrentSail.ID),
	}
}

// equipmentIDsFromVRState converts the option identifiers exposed by the game
// into the stable identifiers understood by the routing API.
func equipmentIDsFromVRState(state vrState) []string {
	result := make([]string, 0, len(state.Options))
	seen := make(map[string]struct{}, len(state.Options))
	for _, option := range state.Options {
		normalized := strings.ToLower(strings.TrimSpace(option))
		normalized = strings.NewReplacer("_", "", "-", "", " ", "").Replace(normalized)
		var id string
		switch normalized {
		case "light":
			id = equipment.Light.String()
		case "heavy":
			id = equipment.Heavy.String()
		case "code0", "codezero", "reach":
			id = equipment.Code0.String()
		case "foil", "foils":
			id = equipment.Foils.String()
		case "hull", "hullpolish":
			id = equipment.HullPolish.String()
		case "winch", "winches", "prowinches":
			id = equipment.ProWinches.String()
		default:
			// The game can also expose sail or automatic-mode values here.
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func validateVRState(state vrState) error {
	if state.Position.Latitude < -90 || state.Position.Latitude > 90 {
		return fmt.Errorf("latitude hors limites: %.6f", state.Position.Latitude)
	}
	if state.Position.Longitude < -180 || state.Position.Longitude > 180 {
		return fmt.Errorf("longitude hors limites: %.6f", state.Position.Longitude)
	}
	if state.Stamina != nil && (*state.Stamina < 0 || *state.Stamina > 100) {
		return fmt.Errorf("stamina hors limites: %.3f", *state.Stamina)
	}
	return nil
}

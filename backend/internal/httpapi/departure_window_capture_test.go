package httpapi

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Rejoue la capture du Trophée Jules Verne avant le départ
// (data/data-record.json) sur les prévisions de data/weather. Ce calcul est
// long et dépend de fichiers locaux : il ne tourne que sur demande.
//
//	VR_DEPARTURE_WINDOW=24 go test ./internal/httpapi/ -run TestDepartureWindowOnTheJulesVerneCapture -v -timeout 60m
//
// La valeur donne l'écart entre deux départs, en heures.
func TestDepartureWindowOnTheJulesVerneCapture(t *testing.T) {
	stepHours := os.Getenv("VR_DEPARTURE_WINDOW")
	if stepHours == "" {
		t.Skip("VR_DEPARTURE_WINDOW non défini")
	}
	root := filepath.Join("..", "..", "..", "data")
	state := julesVerneState(t, filepath.Join(root, "data-record.json"))
	weatherFiles, err := filepath.Glob(filepath.Join(root, "weather", "*.grb2"))
	if err != nil || len(weatherFiles) == 0 {
		t.Skip("aucune prévision dans data/weather")
	}
	previousLand := landGeoJSONPath
	landGeoJSONPath = filepath.Join(root, "land", "natural-earth-10m-land.geojson")
	defer func() { landGeoJSONPath = previousLand }()

	annotateVRStateCheckpoints(&state)
	store := newVRStateStore()
	store.current = &state
	body, _ := json.Marshal(map[string]any{
		"weatherFiles":    weatherFiles,
		"polarPath":       filepath.Join(root, "polars", "ultim_bp_11.json"),
		"destination":     map[string]float64{"latitude": state.Arrival.Latitude, "longitude": state.Arrival.Longitude},
		"owned":           []string{"foils", "light", "heavy", "code0", "hull_polish", "pro_winches"},
		"comparisonHours": 72,
		"stepHours":       json.RawMessage(stepHours),
	})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		runDepartureWindow(response, request, store)
	}))
	defer server.Close()
	started := time.Now()
	response, err := http.Post(server.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	event := ""
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			t.Logf("[%5.0f s] %s %s", time.Since(started).Seconds(), event, strings.TrimPrefix(line, "data: "))
			if event == "error" {
				t.Fatal("le calcul a échoué")
			}
		}
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("statut %d", response.StatusCode)
	}
}

// julesVerneState reproduit l'état publié par l'extension pour un record avant
// le départ : la ligne de départ comme position, les portes du leg.
func julesVerneState(t *testing.T, path string) vrState {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("capture absente : %v", err)
	}
	type point struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	var capture struct {
		Res struct {
			Leg struct {
				Start       point                `json:"start"`
				End         point                `json:"end"`
				Open        struct{ Date int64 } `json:"open"`
				Close       struct{ Date int64 } `json:"close"`
				Checkpoints []struct {
					Group   int    `json:"group"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
					Display string `json:"display"`
					Side    string `json:"side"`
					Start   point  `json:"start"`
					End     point  `json:"end"`
				} `json:"checkpoints"`
			} `json:"leg"`
		} `json:"res"`
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	leg := capture.Res.Leg
	state := vrState{
		RaceID:      "817",
		LegNum:      "1",
		RaceType:    "record",
		PreStart:    true,
		Position:    vrStatePosition{Latitude: leg.Start.Lat, Longitude: leg.Start.Lon},
		Arrival:     &vrStateArrival{Latitude: leg.End.Lat, Longitude: leg.End.Lon, Name: "Brest"},
		StartWindow: &vrStateStartWindow{Open: &leg.Open.Date, Close: &leg.Close.Date},
	}
	for _, checkpoint := range leg.Checkpoints {
		state.Checkpoints = append(state.Checkpoints, vrStateCheckpoint{
			Group: checkpoint.Group, ID: checkpoint.ID, Name: checkpoint.Name, Display: checkpoint.Display, Side: checkpoint.Side,
			Start: vrStatePosition{Latitude: checkpoint.Start.Lat, Longitude: checkpoint.Start.Lon},
			End:   vrStatePosition{Latitude: checkpoint.End.Lat, Longitude: checkpoint.End.Lon},
		})
	}
	return state
}

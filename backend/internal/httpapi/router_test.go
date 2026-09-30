package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func TestHealthReturnsOK(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Errorf(
			"statut HTTP obtenu = %d, attendu = %d",
			response.Code,
			http.StatusOK,
		)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"Content-Type obtenu = %q, attendu = %q",
			contentType,
			"application/json",
		)
	}

	body := strings.TrimSpace(response.Body.String())
	expectedBody := `{"status":"ok"}`

	if body != expectedBody {
		t.Errorf(
			"corps obtenu = %q, attendu = %q",
			body,
			expectedBody,
		)
	}
}

func TestPracticalHeadingsRejectsInvalidRequest(t *testing.T) {
	for _, body := range []string{"{", `{}`} {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/practical-headings",
			bytes.NewBufferString(body),
		)
		response := httptest.NewRecorder()

		NewHandler().ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Errorf("corps %q : statut = %d, attendu %d", body, response.Code, http.StatusBadRequest)
		}
	}
}

func TestHealthRejectsPost(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf(
			"statut HTTP obtenu = %d, attendu = %d",
			response.Code,
			http.StatusMethodNotAllowed,
		)
	}
}

func TestVRStateCanBeStoredAndRead(t *testing.T) {
	payload := `{"raceId":"123.4","userId":"u-1","position":{"latitude":49.8439,"longitude":-5.2075},"stamina":87.5,"options":["heavy","foil"],"fullOptions":false,"sails":[{"id":1,"name":"Jib"},{"id":3,"name":"Staysail"}],"currentSail":{"id":1,"name":"Jib"},"checkpoints":[{"group":1,"id":1,"name":"Porte test","side":"stbd","start":{"latitude":49.8,"longitude":-5.2},"end":{"latitude":49.9,"longitude":-5.2}}]}`
	handler := NewHandler()

	postRequest := httptest.NewRequest(http.MethodPost, "/api/vr/state", bytes.NewBufferString(payload))
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, postRequest)

	if postResponse.Code != http.StatusOK {
		t.Fatalf("POST statut = %d, attendu %d: %s", postResponse.Code, http.StatusOK, postResponse.Body.String())
	}
	if postResponse.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("l'endpoint VR doit autoriser l'appel de l'extension")
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/vr/state", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)

	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET statut = %d, attendu %d", getResponse.Code, http.StatusOK)
	}
	if !strings.Contains(getResponse.Body.String(), `"stamina":87.5`) || !strings.Contains(getResponse.Body.String(), `"longitude":-5.2075`) || !strings.Contains(getResponse.Body.String(), `"name":"Porte test"`) {
		t.Fatalf("état retourné incomplet: %s", getResponse.Body.String())
	}
}

func TestVRStateRejectsInvalidPosition(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/vr/state", bytes.NewBufferString(`{"position":{"latitude":91,"longitude":0}}`))
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("statut = %d, attendu %d", response.Code, http.StatusBadRequest)
	}
}

func TestVRStateEquipmentIDsAreMappedToRoutingIDs(t *testing.T) {
	state := vrState{
		Options: []string{"heavy", "foil", "hull", "proWinches", "reach", "heavy", "auto"},
	}
	got := equipmentIDsFromVRState(state)
	want := []string{"heavy", "foils", "hull_polish", "pro_winches", "code0"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("équipements obtenus = %#v, attendus %#v", got, want)
	}
}

func TestPracticalHeadingsRejectsVRStateUseWhenStateIsMissing(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/practical-headings",
		bytes.NewBufferString(`{"useVirtualRegattaState":true}`),
	)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("statut = %d, attendu %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "aucun état Virtual Regatta reçu") {
		t.Fatalf("erreur retournée inattendue: %s", response.Body.String())
	}
}

func TestPracticalHeadingsInjectsVRStateCheckpointsIntoIsochroneSearch(t *testing.T) {
	originalWeatherLoader := loadWeatherProviderForRequest
	originalPolarLoader := loadPolarForRequest
	originalLandLoader := loadLandForRequest
	originalEvaluate := evaluatePracticalHeadings
	originalBuild := buildIsochroneSearch
	t.Cleanup(func() {
		loadWeatherProviderForRequest = originalWeatherLoader
		loadPolarForRequest = originalPolarLoader
		loadLandForRequest = originalLandLoader
		evaluatePracticalHeadings = originalEvaluate
		buildIsochroneSearch = originalBuild
	})

	loadWeatherProviderForRequest = func(...string) (weather.WeatherProvider, cacheLoadStats, error) {
		return nil, cacheLoadStats{}, nil
	}
	loadPolarForRequest = func(string) (*polar.Polar, bool, error) {
		return &polar.Polar{MaxSpeed: 10}, false, nil
	}
	loadLandForRequest = func(string) (*land.GeoJSONConstraint, bool, error) {
		constraint, err := land.DecodeGeoJSON(strings.NewReader(testLandGeoJSON))
		return constraint, false, err
	}
	evaluatePracticalHeadings = func(
		context.Context,
		weather.WeatherProvider,
		*polar.Polar,
		routing.PracticalHeadingInput,
	) ([]routing.PracticalHeadingCandidate, error) {
		return nil, nil
	}
	var capturedGates []routing.CourseGate
	var capturedInitialStamina simulation.StaminaState
	var capturedInitialNavigation *routing.InitialNavigationState
	var capturedTransitionEvaluator *simulation.TransitionEvaluator
	buildIsochroneSearch = func(
		_ context.Context,
		_ weather.WeatherProvider,
		_ *polar.Polar,
		input routing.IsochroneSearchInput,
	) (routing.IsochroneSearch, error) {
		capturedGates = append([]routing.CourseGate(nil), input.Gates...)
		capturedInitialStamina = input.InitialStamina
		capturedInitialNavigation = input.InitialNavigation
		capturedTransitionEvaluator = input.TransitionEvaluator
		instant := input.Instant.Add(input.StepDuration)
		return routing.IsochroneSearch{
			Origin: input.Position, Destination: input.Destination, StartedAt: input.Instant,
			StepDuration: input.StepDuration, Frontiers: []routing.IsochroneFrontier{
				streamTestFrontier(input.Position, instant, 1, -1),
			}, ArrivalRadius: input.ArrivalRadius, Reached: false,
		}, nil
	}

	handler := NewHandler()
	stateRequest := httptest.NewRequest(http.MethodPost, "/api/vr/state", bytes.NewBufferString(`{"position":{"latitude":-10,"longitude":-10},"stamina":87.5,"heading":110.5,"currentSail":{"id":1,"name":"Jib"},"checkpoints":[{"group":1,"id":1,"name":"Porte VR","side":"stbd","start":{"latitude":-10,"longitude":-9.95},"end":{"latitude":-10.05,"longitude":-9.95}}]}`))
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, stateRequest)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("stockage de l'état: statut = %d, corps = %s", stateResponse.Code, stateResponse.Body.String())
	}

	routeRequest := httptest.NewRequest(http.MethodPost, "/api/practical-headings", bytes.NewBufferString(`{"weatherFiles":["fake"],"polarPath":"fake","position":{"latitude":-10,"longitude":-10},"destination":{"latitude":-10,"longitude":-9.9},"instant":"2026-09-05T07:30:00Z","durationMinutes":10,"stepMinutes":10,"headings":[0],"mode":"quick","useVirtualRegattaState":true}`))
	routeResponse := httptest.NewRecorder()
	handler.ServeHTTP(routeResponse, routeRequest)
	if routeResponse.Code != http.StatusOK {
		t.Fatalf("calcul: statut = %d, corps = %s", routeResponse.Code, routeResponse.Body.String())
	}
	if len(capturedGates) != 1 || capturedGates[0].Name != "Porte VR" || capturedGates[0].Direction != routing.GateLeftToRight {
		t.Fatalf("portes transmises au moteur = %+v", capturedGates)
	}
	if capturedInitialStamina.Level != 87.5 {
		t.Fatalf("stamina initiale transmise au moteur = %.2f, attendue 87.5", capturedInitialStamina.Level)
	}
	if capturedInitialNavigation == nil || capturedInitialNavigation.Heading != 110.5 || capturedInitialNavigation.SailID != 1 {
		t.Fatalf("navigation initiale transmise au moteur = %+v", capturedInitialNavigation)
	}
	if capturedTransitionEvaluator == nil {
		t.Fatal("l'évaluateur de transitions stamina devait être activé avec l'état VR")
	}
	if !strings.Contains(routeResponse.Body.String(), `"virtualRegattaGatesUsed":1`) {
		t.Fatalf("métadonnée de porte absente: %s", routeResponse.Body.String())
	}
	if !strings.Contains(routeResponse.Body.String(), `"initialStamina":87.5`) || !strings.Contains(routeResponse.Body.String(), `"staminaFromVirtualRegatta":true`) {
		t.Fatalf("metadonnees de stamina absentes: %s", routeResponse.Body.String())
	}
}

func TestWeatherRefreshRejectsInvalidJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/weather/refresh", bytes.NewBufferString("{"))
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("statut = %d, attendu %d", response.Code, http.StatusBadRequest)
	}
}

func TestCalculationProfiles(t *testing.T) {
	quick, err := calculationProfileFor("quick")
	if err != nil {
		t.Fatal(err)
	}
	if !quick.IncludeIsochrones || quick.HeadingSpacing != 30 || quick.MaxCandidates != 50 {
		t.Fatalf("profil rapide inattendu: %+v", quick)
	}
	precise, err := calculationProfileFor("precise")
	if err != nil {
		t.Fatal(err)
	}
	if !precise.IncludeIsochrones || precise.HeadingSpacing != 15 || precise.MaxCandidates != 100 {
		t.Fatalf("profil précis inattendu: %+v", precise)
	}
	if _, err := calculationProfileFor("unknown"); err == nil {
		t.Fatal("un profil inconnu devait être refusé")
	}
}

func TestRouteStepPassesDestination(t *testing.T) {
	from := navigation.Position{Latitude: 0, Longitude: 0}
	destination := navigation.Position{Latitude: 0, Longitude: 0.01}
	beyond := navigation.Position{Latitude: 0, Longitude: 0.02}
	before := navigation.Position{Latitude: 0, Longitude: 0.005}

	passes, err := routeStepPassesDestination(from, beyond, destination)
	if err != nil {
		t.Fatal(err)
	}
	if !passes {
		t.Fatal("un point dépassant la destination devait être détecté")
	}
	passes, err = routeStepPassesDestination(from, before, destination)
	if err != nil {
		t.Fatal(err)
	}
	if passes {
		t.Fatal("un point avant la destination ne devait pas être détecté")
	}
}

func TestAdaptiveIsochroneDurationRespectsRequestedLimit(t *testing.T) {
	requested := 240 * time.Minute
	short := adaptiveIsochroneDuration(10, requested, polar.BoatSpeed(40))
	if short >= requested {
		t.Fatalf("une courte destination devait réduire l horizon: %s", short)
	}
	if got := adaptiveIsochroneDuration(1000, requested, polar.BoatSpeed(40)); got != requested {
		t.Fatalf("une longue destination devait conserver la limite: %s", got)
	}
	if got := adaptiveIsochroneDuration(10, requested, 0); got != requested {
		t.Fatalf("une vitesse invalide devait conserver la limite: %s", got)
	}
}

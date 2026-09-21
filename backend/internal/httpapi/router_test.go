package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
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

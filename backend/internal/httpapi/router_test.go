package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

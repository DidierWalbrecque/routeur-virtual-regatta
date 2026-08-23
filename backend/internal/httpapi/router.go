package httpapi

import "net/http"

// NewHandler construit le gestionnaire HTTP de l'application.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	return mux
}

// healthHandler indique que l'API fonctionne.
func healthHandler(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)

	_, _ = response.Write([]byte(`{"status":"ok"}`))
}

package httpapi

import "net/http"

// NewHandler construit le gestionnaire HTTP de l'application.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	vrState := newVRStateStore()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /api/practical-headings", func(response http.ResponseWriter, request *http.Request) {
		runPracticalHeadings(response, request, nil, vrState)
	})
	mux.HandleFunc("POST /api/practical-headings/stream", func(response http.ResponseWriter, request *http.Request) {
		runPracticalHeadings(response, request, &practicalHeadingsStream{}, vrState)
	})
	mux.HandleFunc("GET /api/polars", polarsHandler)
	mux.HandleFunc("POST /api/weather/refresh", weatherRefreshHandler)
	mux.HandleFunc("GET /api/weather/current", weatherCurrentHandler)
	mux.HandleFunc("GET /api/weather/status", weatherCurrentHandler)
	mux.HandleFunc("POST /api/vr/state", vrState.post)
	mux.HandleFunc("GET /api/vr/state", vrState.get)

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Access-Control-Allow-Origin", "*")
		response.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}

// healthHandler indique que l'API fonctionne.
func healthHandler(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)

	_, _ = response.Write([]byte(`{"status":"ok"}`))
}

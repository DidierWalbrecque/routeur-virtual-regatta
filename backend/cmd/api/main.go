package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/httpapi"
)

const serverAddress = ":8080"

func main() {
	server := &http.Server{
		Addr:              serverAddress,
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	httpapi.StartGRIBWatcher(context.Background())

	log.Printf("API disponible sur http://localhost%s", serverAddress)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("arrêt du serveur HTTP : %v", err)
	}
}

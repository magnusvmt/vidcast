package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	version := getEnv("VERSION", "dev")
	addr := getEnv("ADDR", ":8080")
	apiKey := getEnv("API_KEY", "")

	s := newStore()

	// Optional: a deployment with no object storage configured still
	// starts up and serves everything else, just with VOD listing
	// reporting 503 (see registerRecordingsRoutes).
	var recordings recordingsAPI
	if cfg, ok := loadS3Config(os.Getenv); ok {
		recordings = newS3Recordings(cfg)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", newRootHandler(version))
	mux.HandleFunc("GET /healthz", newHealthHandler())
	registerStreamKeyRoutes(mux, s, apiKey)
	registerMediaMTXRoutes(mux, s)
	registerChannelsRoutes(mux, s)
	registerRecordingsRoutes(mux, recordings)

	log.Printf("streams listening on %s (version %s)", addr, version)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

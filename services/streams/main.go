package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	version := getEnv("VERSION", "dev")
	addr := getEnv("ADDR", ":8080")

	s := newStore()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", newRootHandler(version))
	mux.HandleFunc("GET /healthz", newHealthHandler())
	registerStreamKeyRoutes(mux, s)
	registerMediaMTXRoutes(mux, s)
	registerChannelsRoutes(mux, s)

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

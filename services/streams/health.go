package main

import "net/http"

func newRootHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Service string `json:"service"`
			Version string `json:"version"`
		}{Service: "streams", Version: version})
	}
}

// newHealthHandler reports liveness. There's no external dependency to check
// here: the store is in-memory, so if the process is answering requests at
// all, it's healthy.
func newHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
}

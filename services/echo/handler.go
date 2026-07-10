package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func newHandler(version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname, err := os.Hostname()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Hostname string `json:"hostname"`
			Version  string `json:"version"`
		}{
			Hostname: hostname,
			Version:  version,
		})
	})
}

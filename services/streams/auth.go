package main

import (
	"errors"
	"net/http"
)

var ErrUnauthorized = errors.New("unauthorized")

// requireAuth wraps fn with a required Bearer token check. When apiKey is
// empty the check is a no-op, so callers can leave the env var unset during
// development and enable it by setting API_KEY on the deployment.
func requireAuth(apiKey string, fn http.HandlerFunc) http.HandlerFunc {
	if apiKey == "" {
		return fn
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+apiKey {
			writeError(w, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		fn(w, r)
	}
}

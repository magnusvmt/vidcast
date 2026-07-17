package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// authWebhookRequest mirrors the JSON body MediaMTX POSTs to its configured
// authHTTPAddress for every publish/read/etc. request, and (in this service's
// convention) to the runOnUnpublish hook as well. Only the fields this
// service acts on are declared; MediaMTX sends more (ip, user, protocol, id,
// query) that aren't needed here.
type authWebhookRequest struct {
	Action   string `json:"action"`
	Path     string `json:"path"`
	Password string `json:"password"`
}

// registerMediaMTXRoutes wires the endpoints MediaMTX calls into mux:
//
//	POST /mediamtx/auth       external auth webhook (authHTTPAddress)
//	POST /mediamtx/unpublish  runOnUnpublish hook, to detect a stream ending
//
// See README.md for the two supported ways of presenting a stream key
// (password field vs. trailing path segment) and why both are accepted.
func registerMediaMTXRoutes(mux *http.ServeMux, s *store) {
	mux.HandleFunc("POST /mediamtx/auth", newMediaMTXAuthHandler(s))
	mux.HandleFunc("POST /mediamtx/unpublish", newMediaMTXUnpublishHandler(s))
}

func newMediaMTXAuthHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authWebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		// Viewing is unrestricted on this platform; only publishing needs a
		// valid stream key.
		if req.Action != "publish" {
			w.WriteHeader(http.StatusOK)
			return
		}

		key := req.Password
		if key == "" {
			key = lastPathSegment(req.Path)
		}
		slug, ok := s.FindByKey(key)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// slug came straight out of FindByKey, so it's known to exist.
		_ = s.SetLive(slug, true)
		w.WriteHeader(http.StatusOK)
	}
}

func newMediaMTXUnpublishHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authWebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		if slug, ok := resolveChannelFromPath(s, req.Path); ok {
			_ = s.SetLive(slug, false)
		}
		// An unresolvable path is a no-op, not an error: MediaMTX doesn't act
		// on this hook's response, so failing it would accomplish nothing.
		w.WriteHeader(http.StatusOK)
	}
}

// resolveChannelFromPath finds the channel a MediaMTX path refers to, trying
// both conventions this service supports: the trailing segment as a stream
// key, then the trailing segment as a channel slug directly.
func resolveChannelFromPath(s *store, path string) (string, bool) {
	segment := lastPathSegment(path)
	if slug, ok := s.FindByKey(segment); ok {
		return slug, true
	}
	if _, ok := s.Get(segment); ok {
		return segment, true
	}
	return "", false
}

func lastPathSegment(path string) string {
	path = strings.Trim(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

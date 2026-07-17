package main

import "net/http"

// registerStreamKeyRoutes wires the stream-key CRUD endpoints onto mux:
//
//	POST   /channels/{slug}/stream-key  create a key (409 if one exists)
//	GET    /channels/{slug}/stream-key  read metadata, never the secret
//	PUT    /channels/{slug}/stream-key  rotate the key (404 if none exists)
//	DELETE /channels/{slug}/stream-key  revoke the channel and its key
func registerStreamKeyRoutes(mux *http.ServeMux, s *store) {
	mux.HandleFunc("POST /channels/{slug}/stream-key", newCreateStreamKeyHandler(s))
	mux.HandleFunc("GET /channels/{slug}/stream-key", newGetStreamKeyHandler(s))
	mux.HandleFunc("PUT /channels/{slug}/stream-key", newRotateStreamKeyHandler(s))
	mux.HandleFunc("DELETE /channels/{slug}/stream-key", newRevokeStreamKeyHandler(s))
}

func newCreateStreamKeyHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		key, err := s.CreateKey(slug)
		if writeStoreError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, struct {
			Slug      string `json:"slug"`
			StreamKey string `json:"streamKey"`
		}{Slug: slug, StreamKey: key})
	}
}

func newGetStreamKeyHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		ch, ok := s.Get(slug)
		if !ok {
			writeStoreError(w, ErrChannelNotFound)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Slug   string `json:"slug"`
			HasKey bool   `json:"hasKey"`
			Live   bool   `json:"live"`
		}{Slug: ch.Slug, HasKey: ch.HasKey, Live: ch.Live})
	}
}

func newRotateStreamKeyHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		key, err := s.RotateKey(slug)
		if writeStoreError(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Slug      string `json:"slug"`
			StreamKey string `json:"streamKey"`
		}{Slug: slug, StreamKey: key})
	}
}

func newRevokeStreamKeyHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if writeStoreError(w, s.RevokeKey(slug)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

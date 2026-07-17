package main

import "net/http"

// registerChannelsRoutes wires the public live-channel listing endpoint:
//
//	GET /channels  every channel currently live, sorted by slug
func registerChannelsRoutes(mux *http.ServeMux, s *store) {
	mux.HandleFunc("GET /channels", newListChannelsHandler(s))
}

func newListChannelsHandler(s *store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Channels []LiveChannel `json:"channels"`
		}{Channels: s.ListLive()})
	}
}

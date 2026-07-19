package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newChannelsTestMux(s *store) http.Handler {
	mux := http.NewServeMux()
	registerChannelsRoutes(mux, s)
	return mux
}

func TestListChannels_ReturnsOnlyLiveChannels(t *testing.T) {
	s := newStore()
	for _, slug := range []string{"alice", "bob"} {
		if _, err := s.CreateKey(slug); err != nil {
			t.Fatalf("CreateKey(%s) error = %v", slug, err)
		}
	}
	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive() error = %v", err)
	}
	mux := newChannelsTestMux(s)

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Channels []struct {
			Slug string `json:"slug"`
			Live bool   `json:"live"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if len(body.Channels) != 1 || body.Channels[0].Slug != "alice" || !body.Channels[0].Live {
		t.Fatalf("channels = %+v, want [{alice true}]", body.Channels)
	}

	// Assert the raw wire format explicitly: json.Unmarshal matches field
	// names case-insensitively when a struct has no tag, so decoding into a
	// tagged struct above wouldn't catch a handler that emits "Slug"/"Live"
	// instead of "slug"/"live".
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"slug":"alice"`)) {
		t.Errorf("response body missing lowercase \"slug\" key: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"live":true`)) {
		t.Errorf("response body missing lowercase \"live\" key: %s", rec.Body.String())
	}
}

func TestListChannels_ReturnsEmptyListWhenNoneAreLive(t *testing.T) {
	mux := newChannelsTestMux(newStore())

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Channels []json.RawMessage `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if body.Channels == nil {
		t.Fatal("channels field is null, want an empty array")
	}
	if len(body.Channels) != 0 {
		t.Fatalf("channels = %v, want empty", body.Channels)
	}
}

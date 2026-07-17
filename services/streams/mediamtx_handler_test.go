package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMediaMTXTestMux(s *store) http.Handler {
	mux := http.NewServeMux()
	registerMediaMTXRoutes(mux, s)
	return mux
}

func postAuth(t *testing.T, mux http.Handler, req authWebhookRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq := httptest.NewRequest(http.MethodPost, "/mediamtx/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httpReq)
	return rec
}

func TestMediaMTXAuth_AuthorizesPublishWithValidKeyInPasswordField(t *testing.T) {
	s := newStore()
	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newMediaMTXTestMux(s)

	rec := postAuth(t, mux, authWebhookRequest{Action: "publish", Path: "alice", Password: key})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ch, _ := s.Get("alice"); !ch.Live {
		t.Error("channel not marked live after authorized publish")
	}
}

func TestMediaMTXAuth_AuthorizesPublishWithKeyEmbeddedInPath(t *testing.T) {
	s := newStore()
	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newMediaMTXTestMux(s)

	rec := postAuth(t, mux, authWebhookRequest{Action: "publish", Path: "live/" + key})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ch, _ := s.Get("alice"); !ch.Live {
		t.Error("channel not marked live after authorized publish")
	}
}

func TestMediaMTXAuth_RejectsPublishWithUnknownKey(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newMediaMTXTestMux(s)

	rec := postAuth(t, mux, authWebhookRequest{Action: "publish", Path: "alice", Password: "sk_totally-wrong"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if ch, _ := s.Get("alice"); ch.Live {
		t.Error("channel marked live after rejected publish")
	}
}

func TestMediaMTXAuth_RejectsPublishWithNoKeyAtAll(t *testing.T) {
	s := newStore()
	mux := newMediaMTXTestMux(s)

	rec := postAuth(t, mux, authWebhookRequest{Action: "publish", Path: "alice"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestMediaMTXAuth_AuthorizesReadActionsWithoutAKey(t *testing.T) {
	s := newStore()
	mux := newMediaMTXTestMux(s)

	for _, action := range []string{"read", "playback"} {
		rec := postAuth(t, mux, authWebhookRequest{Action: action, Path: "alice"})
		if rec.Code != http.StatusOK {
			t.Errorf("action %q: status = %d, want %d", action, rec.Code, http.StatusOK)
		}
	}
}

func TestMediaMTXUnpublish_MarksChannelOfflineByPath(t *testing.T) {
	s := newStore()
	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive() error = %v", err)
	}
	mux := newMediaMTXTestMux(s)

	body, _ := json.Marshal(authWebhookRequest{Path: "live/" + key})
	req := httptest.NewRequest(http.MethodPost, "/mediamtx/unpublish", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ch, _ := s.Get("alice"); ch.Live {
		t.Error("channel still live after unpublish")
	}
}

func TestMediaMTXUnpublish_MarksChannelOfflineWhenPathIsBareSlug(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive() error = %v", err)
	}
	mux := newMediaMTXTestMux(s)

	body, _ := json.Marshal(authWebhookRequest{Path: "alice"})
	req := httptest.NewRequest(http.MethodPost, "/mediamtx/unpublish", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ch, _ := s.Get("alice"); ch.Live {
		t.Error("channel still live after unpublish")
	}
}

func TestMediaMTXUnpublish_UnknownPathIsANoOp(t *testing.T) {
	s := newStore()
	mux := newMediaMTXTestMux(s)

	body, _ := json.Marshal(authWebhookRequest{Path: "nobody"})
	req := httptest.NewRequest(http.MethodPost, "/mediamtx/unpublish", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestMux(s *store) http.Handler {
	mux := http.NewServeMux()
	registerStreamKeyRoutes(mux, s, "")
	return mux
}

func TestCreateStreamKey_ReturnsGeneratedKeyOn201(t *testing.T) {
	mux := newTestMux(newStore())

	req := httptest.NewRequest(http.MethodPost, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body struct {
		Slug      string `json:"slug"`
		StreamKey string `json:"streamKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if body.Slug != "alice" {
		t.Errorf("slug = %q, want %q", body.Slug, "alice")
	}
	if body.StreamKey == "" {
		t.Error("streamKey is empty, want a generated key")
	}
}

func TestCreateStreamKey_ConflictWhenChannelAlreadyHasKey(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newTestMux(s)

	req := httptest.NewRequest(http.MethodPost, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestGetStreamKey_ReturnsMetadataWithoutSecret(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newTestMux(s)

	req := httptest.NewRequest(http.MethodGet, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if bodyContainsKey := jsonHasField(t, rec.Body.Bytes(), "streamKey"); bodyContainsKey {
		t.Error("GET response leaked the streamKey secret")
	}

	var body struct {
		Slug   string `json:"slug"`
		HasKey bool   `json:"hasKey"`
		Live   bool   `json:"live"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if !body.HasKey {
		t.Error("hasKey = false, want true")
	}
	if body.Live {
		t.Error("live = true, want false before any publish")
	}
}

func TestGetStreamKey_NotFoundForUnknownChannel(t *testing.T) {
	mux := newTestMux(newStore())

	req := httptest.NewRequest(http.MethodGet, "/channels/nobody/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestRotateStreamKey_ReplacesKeyAndInvalidatesOld(t *testing.T) {
	s := newStore()
	oldKey, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newTestMux(s)

	req := httptest.NewRequest(http.MethodPut, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		StreamKey string `json:"streamKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.StreamKey == "" || body.StreamKey == oldKey {
		t.Errorf("streamKey = %q, want a new non-empty key different from %q", body.StreamKey, oldKey)
	}
	if _, ok := s.FindByKey(oldKey); ok {
		t.Error("old key still resolves after rotation")
	}
}

func TestRotateStreamKey_NotFoundForUnknownChannel(t *testing.T) {
	mux := newTestMux(newStore())

	req := httptest.NewRequest(http.MethodPut, "/channels/nobody/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestRevokeStreamKey_DeletesChannelOn204(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	mux := newTestMux(s)

	req := httptest.NewRequest(http.MethodDelete, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if _, ok := s.Get("alice"); ok {
		t.Error("channel still exists after revoke")
	}
}

func TestRevokeStreamKey_NotFoundForUnknownChannel(t *testing.T) {
	mux := newTestMux(newStore())

	req := httptest.NewRequest(http.MethodDelete, "/channels/nobody/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAuthMiddleware_AllowsValidBearerToken(t *testing.T) {
	mux := http.NewServeMux()
	registerStreamKeyRoutes(mux, newStore(), "secret123")

	req := httptest.NewRequest(http.MethodPost, "/channels/alice/stream-key", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestAuthMiddleware_RejectsMissingBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		wantErr string
	}{
		{"POST", http.MethodPost, "/channels/alice/stream-key", "unauthorized"},
		{"GET", http.MethodGet, "/channels/alice/stream-key", "unauthorized"},
		{"PUT", http.MethodPut, "/channels/alice/stream-key", "unauthorized"},
		{"DELETE", http.MethodDelete, "/channels/alice/stream-key", "unauthorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerStreamKeyRoutes(mux, newStore(), "secret123")

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		})
	}
}

func TestAuthMiddleware_RejectsWrongBearerToken(t *testing.T) {
	mux := http.NewServeMux()
	registerStreamKeyRoutes(mux, newStore(), "secret123")

	req := httptest.NewRequest(http.MethodPost, "/channels/alice/stream-key", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestAuthMiddleware_NoopWhenApiKeyEmpty(t *testing.T) {
	mux := http.NewServeMux()
	registerStreamKeyRoutes(mux, newStore(), "")

	req := httptest.NewRequest(http.MethodPost, "/channels/alice/stream-key", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func jsonHasField(t *testing.T, body []byte, field string) bool {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, body)
	}
	_, ok := raw[field]
	return ok
}

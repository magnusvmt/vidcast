package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRootHandler_ReportsServiceIdentityAndVersion(t *testing.T) {
	handler := newRootHandler("v1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Service string `json:"service"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if body.Service != "chat" {
		t.Errorf("service = %q, want %q", body.Service, "chat")
	}
	if body.Version != "v1.2.3" {
		t.Errorf("version = %q, want %q", body.Version, "v1.2.3")
	}
}

func TestHealthHandler_OKWhenRedisReachable(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	handler := newHealthHandler([]*redis.Client{rdb})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v (body: %s)", err, rec.Body.String())
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
}

func TestHealthHandler_ServiceUnavailableWhenRedisUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.Close()

	handler := newHealthHandler([]*redis.Client{rdb})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

// TestHealthHandler_OKWhenAllShardsReachable proves the health check covers
// every configured shard, not just the first, once chat is sharded across
// multiple Redis instances.
func TestHealthHandler_OKWhenAllShardsReachable(t *testing.T) {
	var shards []*redis.Client
	for i := 0; i < 3; i++ {
		mr := miniredis.RunT(t)
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
		shards = append(shards, rdb)
	}

	handler := newHealthHandler(shards)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// TestHealthHandler_ServiceUnavailableWhenAnyShardUnreachable proves a
// single down shard is enough to fail readiness even though the other
// shards are healthy - a partially reachable shard set means some rooms
// are unreachable, so the pod should not be marked Ready.
func TestHealthHandler_ServiceUnavailableWhenAnyShardUnreachable(t *testing.T) {
	mr1 := miniredis.RunT(t)
	rdb1 := redis.NewClient(&redis.Options{Addr: mr1.Addr()})
	t.Cleanup(func() { _ = rdb1.Close() })

	mr2 := miniredis.RunT(t)
	rdb2 := redis.NewClient(&redis.Options{Addr: mr2.Addr()})
	t.Cleanup(func() { _ = rdb2.Close() })
	mr2.Close()

	handler := newHealthHandler([]*redis.Client{rdb1, rdb2})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

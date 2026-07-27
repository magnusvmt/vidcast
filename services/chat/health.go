package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

func newRootHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Service string `json:"service"`
			Version string `json:"version"`
		}{Service: "chat", Version: version})
	}
}

// newHealthHandler reports healthy only when every shard responds: a chat
// pod with one unreachable shard can't serve every room, so it shouldn't be
// marked Ready either.
func newHealthHandler(shards []*redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := pingAllShards(ctx, shards); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}{Status: "error", Error: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
}

func pingAllShards(ctx context.Context, shards []*redis.Client) error {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for _, shard := range shards {
		wg.Add(1)
		go func(shard *redis.Client) {
			defer wg.Done()
			if err := shard.Ping(ctx).Err(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(shard)
	}
	wg.Wait()
	return firstErr
}

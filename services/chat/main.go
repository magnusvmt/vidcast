package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

func main() {
	version := getEnv("VERSION", "dev")
	addr := getEnv("ADDR", ":8080")
	redisAddrs := getRedisAddrs()

	shards := make([]*redis.Client, len(redisAddrs))
	for i, redisAddr := range redisAddrs {
		shards[i] = redis.NewClient(&redis.Options{Addr: redisAddr})
	}
	defer func() {
		for _, shard := range shards {
			_ = shard.Close()
		}
	}()

	broker := newBroker(shards...)
	hub := newHub(context.Background(), broker)

	mux := http.NewServeMux()
	mux.HandleFunc("/", newRootHandler(version))
	mux.HandleFunc("/healthz", newHealthHandler(shards))
	mux.HandleFunc("/ws", newWebSocketHandler(hub, broker))

	log.Printf("chat listening on %s (version %s, redis shards %v)", addr, version, redisAddrs)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// getRedisAddrs reads REDIS_ADDR as a comma-separated list of Redis
// instance addresses. Chat rooms are hashed across however many addresses
// are given (see shardIndex), so scaling pub/sub beyond a single Redis
// instance is a matter of listing more addresses here - no code change.
func getRedisAddrs() []string {
	raw := getEnv("REDIS_ADDR", "localhost:6379")
	var addrs []string
	for _, addr := range strings.Split(raw, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

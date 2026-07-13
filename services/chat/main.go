package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

func main() {
	version := getEnv("VERSION", "dev")
	addr := getEnv("ADDR", ":8080")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	broker := newBroker(rdb)
	hub := newHub(context.Background(), broker)

	mux := http.NewServeMux()
	mux.HandleFunc("/", newRootHandler(version))
	mux.HandleFunc("/healthz", newHealthHandler(rdb))
	mux.HandleFunc("/ws", newWebSocketHandler(hub, broker))

	log.Printf("chat listening on %s (version %s, redis %s)", addr, version, redisAddr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package main

import (
	"os"
	"reflect"
	"testing"
)

func withRedisAddrEnv(t *testing.T, value string) {
	t.Helper()
	t.Setenv("REDIS_ADDR", value)
}

func TestGetRedisAddrs_DefaultsToLocalhostWhenUnset(t *testing.T) {
	_ = os.Unsetenv("REDIS_ADDR")

	got := getRedisAddrs()
	want := []string{"localhost:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getRedisAddrs() = %v, want %v", got, want)
	}
}

func TestGetRedisAddrs_SingleAddress(t *testing.T) {
	withRedisAddrEnv(t, "redis:6379")

	got := getRedisAddrs()
	want := []string{"redis:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getRedisAddrs() = %v, want %v", got, want)
	}
}

func TestGetRedisAddrs_SplitsCommaSeparatedShardList(t *testing.T) {
	withRedisAddrEnv(t, "redis-0:6379,redis-1:6379,redis-2:6379")

	got := getRedisAddrs()
	want := []string{"redis-0:6379", "redis-1:6379", "redis-2:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getRedisAddrs() = %v, want %v", got, want)
	}
}

func TestGetRedisAddrs_TrimsWhitespaceAndDropsEmptyEntries(t *testing.T) {
	withRedisAddrEnv(t, " redis-0:6379 , , redis-1:6379,")

	got := getRedisAddrs()
	want := []string{"redis-0:6379", "redis-1:6379"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getRedisAddrs() = %v, want %v", got, want)
	}
}

func TestGetRedisAddrs_ReturnsEmptyWhenAllEntriesAreEmpty(t *testing.T) {
	withRedisAddrEnv(t, ",")

	got := getRedisAddrs()
	if len(got) != 0 {
		t.Fatalf("getRedisAddrs() = %v, want empty slice for REDIS_ADDR=\",\"", got)
	}
}

func TestGetRedisAddrs_ReturnsEmptyForWhitespaceOnly(t *testing.T) {
	withRedisAddrEnv(t, "  ")

	got := getRedisAddrs()
	if len(got) != 0 {
		t.Fatalf("getRedisAddrs() = %v, want empty slice for whitespace-only REDIS_ADDR", got)
	}
}

package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestBroker(t *testing.T) (*Broker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return newBroker(rdb), mr
}

func TestBroker_SubscribeReceivesPublishedMessage(t *testing.T) {
	broker, _ := newTestBroker(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan []byte, 1)
	broker.subscribe(ctx, "room-a", func(payload []byte) {
		received <- payload
	})

	// Give the subscription time to establish before publishing - Redis
	// pub/sub only delivers to subscribers already listening when a message
	// is published.
	time.Sleep(50 * time.Millisecond)

	if err := broker.publish(context.Background(), "room-a", []byte("hello")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case payload := <-received:
		if string(payload) != "hello" {
			t.Fatalf("received %q, want %q", payload, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never received the published message")
	}
}

func TestBroker_SubscribeIgnoresOtherRoomChannels(t *testing.T) {
	broker, _ := newTestBroker(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan []byte, 1)
	broker.subscribe(ctx, "room-a", func(payload []byte) {
		received <- payload
	})
	time.Sleep(50 * time.Millisecond)

	if err := broker.publish(context.Background(), "room-b", []byte("hello")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case payload := <-received:
		t.Fatalf("subscriber to room-a should not receive room-b's message, got %q", payload)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBroker_StopsDeliveringAfterContextCancelled(t *testing.T) {
	broker, _ := newTestBroker(t)
	ctx, cancel := context.WithCancel(context.Background())

	received := make(chan []byte, 1)
	broker.subscribe(ctx, "room-a", func(payload []byte) {
		received <- payload
	})
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	if err := broker.publish(context.Background(), "room-a", []byte("hello")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case payload := <-received:
		t.Fatalf("cancelled subscription should not receive messages, got %q", payload)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestShardIndex_DeterministicForSameRoom(t *testing.T) {
	first := shardIndex("stream-42", 5)
	second := shardIndex("stream-42", 5)
	if first != second {
		t.Fatalf("shardIndex(%q, 5) = %d then %d, want the same shard both times", "stream-42", first, second)
	}
}

func TestShardIndex_WithinRange(t *testing.T) {
	rooms := []string{"room-a", "room-b", "stream-42", "general", "help-desk", "vip-lounge"}
	const n = 4
	for _, room := range rooms {
		idx := shardIndex(room, n)
		if idx < 0 || idx >= n {
			t.Fatalf("shardIndex(%q, %d) = %d, want in range [0, %d)", room, n, idx, n)
		}
	}
}

func TestShardIndex_DistributesAcrossMultipleShards(t *testing.T) {
	rooms := []string{"room-a", "room-b", "room-c", "room-d", "room-e", "room-f", "room-g", "room-h"}
	const n = 4
	seen := make(map[int]bool)
	for _, room := range rooms {
		seen[shardIndex(room, n)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("shardIndex spread only %d distinct shard(s) across %d rooms with n=%d, want more than 1 (routing every room to the same shard defeats sharding)", len(seen), len(rooms), n)
	}
}

func TestShardIndex_SingleShardAlwaysZero(t *testing.T) {
	for _, room := range []string{"room-a", "room-b", "stream-42"} {
		if idx := shardIndex(room, 1); idx != 0 {
			t.Fatalf("shardIndex(%q, 1) = %d, want 0", room, idx)
		}
	}
}

// TestBroker_MultiShard_PublishSubscribeRoundTrip proves rooms sharded
// across several independent Redis instances still deliver correctly: each
// room is routed to a single shard for both subscribe and publish, so a
// subscriber only ever needs to be listening on the shard its room hashes
// to for the message to arrive.
func TestBroker_MultiShard_PublishSubscribeRoundTrip(t *testing.T) {
	const shardCount = 3
	shards := make([]*redis.Client, shardCount)
	for i := range shards {
		mr := miniredis.RunT(t)
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = rdb.Close() })
		shards[i] = rdb
	}
	broker := newBroker(shards...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rooms := []string{"room-a", "room-b", "room-c", "room-d", "room-e", "room-f"}
	received := make(map[string]chan []byte, len(rooms))
	for _, room := range rooms {
		room := room
		ch := make(chan []byte, 1)
		received[room] = ch
		broker.subscribe(ctx, room, func(payload []byte) { ch <- payload })
	}
	time.Sleep(50 * time.Millisecond)

	for _, room := range rooms {
		if err := broker.publish(context.Background(), room, []byte("hello "+room)); err != nil {
			t.Fatalf("publish to %q: %v", room, err)
		}
	}

	for _, room := range rooms {
		select {
		case payload := <-received[room]:
			want := "hello " + room
			if string(payload) != want {
				t.Fatalf("room %q received %q, want %q", room, payload, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("room %q never received its published message across shards", room)
		}
	}
}

// TestBroker_FansOutAcrossIndependentClients is the core acceptance test for
// this service: two Broker instances backed by two separate Redis clients
// (standing in for two stateless chat pods) both connected to the same
// Redis, subscribed to the same room, both receive a message published by
// either one - proving fan-out works without pods sharing any local state.
func TestBroker_FansOutAcrossIndependentClients(t *testing.T) {
	mr := miniredis.RunT(t)

	rdbPodA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdbPodA.Close() })
	rdbPodB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdbPodB.Close() })

	brokerPodA := newBroker(rdbPodA)
	brokerPodB := newBroker(rdbPodB)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receivedOnA := make(chan []byte, 1)
	receivedOnB := make(chan []byte, 1)
	brokerPodA.subscribe(ctx, "stream-42", func(payload []byte) { receivedOnA <- payload })
	brokerPodB.subscribe(ctx, "stream-42", func(payload []byte) { receivedOnB <- payload })
	time.Sleep(50 * time.Millisecond)

	if err := brokerPodA.publish(context.Background(), "stream-42", []byte("hi from pod A")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case payload := <-receivedOnA:
		if string(payload) != "hi from pod A" {
			t.Fatalf("pod A received %q, want %q", payload, "hi from pod A")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publishing pod never received its own message back")
	}

	select {
	case payload := <-receivedOnB:
		if string(payload) != "hi from pod A" {
			t.Fatalf("pod B received %q, want %q", payload, "hi from pod A")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pod B never received the message published on pod A - cross-pod fan-out is broken")
	}
}

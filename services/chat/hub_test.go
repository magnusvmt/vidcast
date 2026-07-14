package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeSubscriber stands in for the Redis-backed broker so Hub's local
// fan-out and room-lifecycle bookkeeping can be tested without a Redis
// server. It records subscribe/cancel calls and lets tests trigger
// "messages arriving from Redis" by invoking the captured callback.
type fakeSubscriber struct {
	mu          sync.Mutex
	subscribes  []string
	cancels     []string
	onMessageFn map[string]func([]byte)
}

func newFakeSubscriber() *fakeSubscriber {
	return &fakeSubscriber{onMessageFn: make(map[string]func([]byte))}
}

func (f *fakeSubscriber) subscribe(ctx context.Context, room string, onMessage func([]byte)) {
	f.mu.Lock()
	f.subscribes = append(f.subscribes, room)
	f.onMessageFn[room] = onMessage
	f.mu.Unlock()

	go func() {
		<-ctx.Done()
		f.mu.Lock()
		f.cancels = append(f.cancels, room)
		f.mu.Unlock()
	}()
}

func (f *fakeSubscriber) deliver(room string, payload []byte) {
	f.mu.Lock()
	fn := f.onMessageFn[room]
	f.mu.Unlock()
	fn(payload)
}

func (f *fakeSubscriber) subscribeCount(room string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.subscribes {
		if r == room {
			n++
		}
	}
	return n
}

func (f *fakeSubscriber) cancelCountEventually(t *testing.T, room string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		n := 0
		for _, r := range f.cancels {
			if r == room {
				n++
			}
		}
		f.mu.Unlock()
		if n >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("room %q: expected at least %d cancellation(s), got fewer", room, want)
}

func TestHub_Join_SubscribesOnlyOnceForFirstClientInRoom(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	b := &client{send: make(chan []byte, 1)}
	h.join("room-a", a)
	h.join("room-a", b)

	if got := sub.subscribeCount("room-a"); got != 1 {
		t.Fatalf("subscribeCount(room-a) = %d, want 1 (second joiner should reuse the existing subscription)", got)
	}
}

func TestHub_Leave_UnsubscribesOnlyWhenRoomBecomesEmpty(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	b := &client{send: make(chan []byte, 1)}
	h.join("room-a", a)
	h.join("room-a", b)

	h.leave("room-a", a)
	time.Sleep(20 * time.Millisecond)
	if got := len(sub.cancels); got != 0 {
		t.Fatalf("cancels after first leave = %d, want 0 (room still has a client)", got)
	}

	h.leave("room-a", b)
	sub.cancelCountEventually(t, "room-a", 1)
}

func TestHub_Join_ResubscribesAfterRoomEmptiesAndRefills(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	h.join("room-a", a)
	h.leave("room-a", a)
	sub.cancelCountEventually(t, "room-a", 1)

	b := &client{send: make(chan []byte, 1)}
	h.join("room-a", b)

	if got := sub.subscribeCount("room-a"); got != 2 {
		t.Fatalf("subscribeCount(room-a) = %d, want 2 (a fresh client should trigger a fresh subscription)", got)
	}
}

func TestHub_BroadcastLocal_DeliversOnlyToClientsInRoom(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	b := &client{send: make(chan []byte, 1)}
	h.join("room-a", a)
	h.join("room-b", b)

	sub.deliver("room-a", []byte("hello"))

	select {
	case msg := <-a.send:
		if string(msg) != "hello" {
			t.Fatalf("client in room-a received %q, want %q", msg, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("expected client in room-a to receive the message")
	}

	select {
	case msg := <-b.send:
		t.Fatalf("client in room-b should not receive room-a's message, got %q", msg)
	default:
	}
}

func TestHub_BroadcastLocal_SkipsClientAfterLeave(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	h.join("room-a", a)
	h.leave("room-a", a)

	// Must not panic even though no clients remain and the room map entry
	// has been removed.
	h.broadcastLocal("room-a", []byte("hello"))
}

// TestHub_Join_ConcurrentFirstJoinersDoNotRaceOnRoomCreation exercises two
// goroutines calling join for the same not-yet-existing room at the same
// time - e.g. two viewers connecting to a brand-new stream simultaneously.
// Under -race, a room.clients write left outside Hub's lock would be
// flagged as a concurrent map write here.
func TestHub_Join_ConcurrentFirstJoinersDoNotRaceOnRoomCreation(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte, 1)}
	b := &client{send: make(chan []byte, 1)}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); h.join("room-a", a) }()
	go func() { defer wg.Done(); h.join("room-a", b) }()
	wg.Wait()

	if got := sub.subscribeCount("room-a"); got != 1 {
		t.Fatalf("subscribeCount(room-a) = %d, want 1 (only the first joiner should create the subscription)", got)
	}
}

func TestHub_BroadcastLocal_DoesNotBlockOnFullClientBuffer(t *testing.T) {
	sub := newFakeSubscriber()
	h := newHub(context.Background(), sub)

	a := &client{send: make(chan []byte)} // unbuffered, nobody reads it
	h.join("room-a", a)

	done := make(chan struct{})
	go func() {
		h.broadcastLocal("room-a", []byte("hello"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("broadcastLocal blocked on a slow/unread client instead of dropping the message")
	}
}

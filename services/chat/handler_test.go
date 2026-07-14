package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
)

func newTestServer(t *testing.T, redisAddr string) *httptest.Server {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	broker := newBroker(rdb)
	hub := newHub(context.Background(), broker)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", newWebSocketHandler(hub, broker))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func dialChat(t *testing.T, srv *httptest.Server, room, username string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=" + room + "&username=" + username
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("invalid JSON message: %v (body: %s)", err, data)
	}
	return msg
}

func expectNoMessage(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err == nil {
		t.Fatalf("expected no message, got %q", data)
	}
}

func TestWebSocketHandler_BroadcastsToOtherClientsInSameRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestServer(t, mr.Addr())

	alice := dialChat(t, srv, "stream-1", "alice")
	bob := dialChat(t, srv, "stream-1", "bob")
	time.Sleep(50 * time.Millisecond) // let both joins subscribe before publishing

	if err := alice.Write(context.Background(), websocket.MessageText, []byte("hello room")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := readMessage(t, bob)
	if got.Body != "hello room" {
		t.Errorf("body = %q, want %q", got.Body, "hello room")
	}
	if got.Username != "alice" {
		t.Errorf("username = %q, want %q", got.Username, "alice")
	}
	if got.Room != "stream-1" {
		t.Errorf("room = %q, want %q", got.Room, "stream-1")
	}
}

func TestWebSocketHandler_SenderReceivesOwnMessage(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestServer(t, mr.Addr())

	alice := dialChat(t, srv, "stream-1", "alice")
	time.Sleep(50 * time.Millisecond)

	if err := alice.Write(context.Background(), websocket.MessageText, []byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := readMessage(t, alice)
	if got.Body != "hi" {
		t.Errorf("body = %q, want %q", got.Body, "hi")
	}
}

func TestWebSocketHandler_DoesNotDeliverAcrossDifferentRooms(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestServer(t, mr.Addr())

	alice := dialChat(t, srv, "stream-1", "alice")
	carol := dialChat(t, srv, "stream-2", "carol")
	time.Sleep(50 * time.Millisecond)

	if err := alice.Write(context.Background(), websocket.MessageText, []byte("hello room 1")); err != nil {
		t.Fatalf("write: %v", err)
	}

	readMessage(t, alice) // sender's own echo
	expectNoMessage(t, carol)
}

// TestWebSocketHandler_IdleReadOnlyClientStaysConnected asserts a client
// that never sends a message (e.g. watching chat without typing) stays
// connected: conn.Read must not be wrapped in a short-lived context, since
// coder/websocket closes the whole connection - not just that call - when a
// Read's context deadline fires. Liveness is the ping/pong loop's job alone.
func TestWebSocketHandler_IdleReadOnlyClientStaysConnected(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestServer(t, mr.Addr())

	alice := dialChat(t, srv, "stream-1", "alice")
	bob := dialChat(t, srv, "stream-1", "bob")
	time.Sleep(50 * time.Millisecond)

	// Bob never sends anything. If Read were ever wrapped in a short
	// per-call timeout again, this wait would be enough to trip it and
	// close bob's connection out from under him.
	time.Sleep(300 * time.Millisecond)

	if err := alice.Write(context.Background(), websocket.MessageText, []byte("still there?")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readMessage(t, bob)
	if got.Body != "still there?" {
		t.Errorf("body = %q, want %q", got.Body, "still there?")
	}
}

func TestWebSocketHandler_RequiresRoomQueryParameter(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestServer(t, mr.Addr())

	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestWebSocketHandler_FansOutAcrossTwoServers is the end-to-end acceptance
// test for horizontal scaling: two independent httptest servers (standing in
// for two stateless chat pods behind a load balancer) share one Redis. A
// client connected to server A sends a message; a client connected to
// server B - which never touched server A directly - receives it purely
// through Redis pub/sub.
func TestWebSocketHandler_FansOutAcrossTwoServers(t *testing.T) {
	mr := miniredis.RunT(t)
	podA := newTestServer(t, mr.Addr())
	podB := newTestServer(t, mr.Addr())

	alice := dialChat(t, podA, "stream-1", "alice")
	bob := dialChat(t, podB, "stream-1", "bob")
	time.Sleep(50 * time.Millisecond)

	if err := alice.Write(context.Background(), websocket.MessageText, []byte("hello from pod A")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := readMessage(t, bob)
	if got.Body != "hello from pod A" {
		t.Errorf("body = %q, want %q", got.Body, "hello from pod A")
	}
}

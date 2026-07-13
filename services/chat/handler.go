package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// newWebSocketHandler upgrades the connection and bridges it to the room:
// inbound client messages are published to Redis (never broadcast locally
// directly), and outbound messages are whatever the room's Redis
// subscription delivers back through the hub - including the sender's own
// message. That single code path is what keeps a single pod's clients and
// every other pod's clients in sync.
func newWebSocketHandler(hub *Hub, broker *Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		room := r.URL.Query().Get("room")
		if room == "" {
			http.Error(w, "room query parameter is required", http.StatusBadRequest)
			return
		}
		username := r.URL.Query().Get("username")
		if username == "" {
			username = "anonymous"
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		ctx, stop := context.WithCancel(r.Context())
		defer stop()

		c := &client{send: make(chan []byte, 16)}
		hub.join(room, c)

		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			for payload := range c.send {
				writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := conn.Write(writeCtx, websocket.MessageText, payload)
				cancel()
				if err != nil {
					return
				}
			}
		}()

		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := conn.Ping(pingCtx)
					cancel()
					if err != nil {
						stop()
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			readCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			_, data, err := conn.Read(readCtx)
			cancel()
			if err != nil {
				break
			}

			msg := Message{Room: room, Username: username, Body: string(data), SentAt: time.Now().UTC()}
			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := broker.publish(ctx, room, payload); err != nil {
				log.Printf("chat: publish to room %q failed: %v", room, err)
			}
		}

		// hub.leave must complete before closing c.send: broadcastLocal and
		// leave share a mutex, so once leave returns no future broadcast can
		// reach this client and it's safe to close its channel.
		hub.leave(room, c)
		close(c.send)
		<-writeDone

		conn.Close(websocket.StatusNormalClosure, "")
	}
}

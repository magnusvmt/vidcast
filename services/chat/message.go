package main

import "time"

// Message is the wire format exchanged over WebSocket connections and
// carried as the payload of Redis pub/sub messages, so every pod publishing
// or receiving it must agree on this shape.
type Message struct {
	Room     string    `json:"room"`
	Username string    `json:"username"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sentAt"`
}

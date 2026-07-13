package main

import (
	"context"
	"sync"
)

// client is one local WebSocket connection. Messages destined for it are
// written to send by broadcastLocal and read out by the connection's writer
// goroutine.
type client struct {
	send chan []byte
}

// roomSubscriber is the Redis pub/sub side of a room: subscribe starts
// listening on the room's channel and invokes onMessage for every payload
// received, until ctx is cancelled. Broker implements this; tests use a
// fake so Hub's bookkeeping can be verified without a Redis server.
type roomSubscriber interface {
	subscribe(ctx context.Context, room string, onMessage func([]byte))
}

type room struct {
	clients map[*client]struct{}
	cancel  context.CancelFunc
}

// Hub tracks, per room, the WebSocket clients connected to this pod. A pod
// only ever needs one Redis subscription per room regardless of how many
// local clients are in it, so Hub opens that subscription when the first
// local client joins and tears it down when the last one leaves.
type Hub struct {
	mu     sync.Mutex
	ctx    context.Context
	broker roomSubscriber
	rooms  map[string]*room
}

func newHub(ctx context.Context, broker roomSubscriber) *Hub {
	return &Hub{
		ctx:    ctx,
		broker: broker,
		rooms:  make(map[string]*room),
	}
}

func (h *Hub) join(roomName string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[roomName]
	if !ok {
		subCtx, cancel := context.WithCancel(h.ctx)
		r = &room{clients: make(map[*client]struct{}), cancel: cancel}
		h.rooms[roomName] = r
		h.broker.subscribe(subCtx, roomName, func(payload []byte) {
			h.broadcastLocal(roomName, payload)
		})
	}
	r.clients[c] = struct{}{}
}

func (h *Hub) leave(roomName string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[roomName]
	if !ok {
		return
	}
	delete(r.clients, c)
	if len(r.clients) == 0 {
		r.cancel()
		delete(h.rooms, roomName)
	}
}

// broadcastLocal delivers payload to every client currently registered for
// roomName on this pod. It never blocks: a client whose send buffer is full
// has the message dropped for it rather than stalling delivery to everyone
// else in the room.
func (h *Hub) broadcastLocal(roomName string, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[roomName]
	if !ok {
		return
	}
	for c := range r.clients {
		select {
		case c.send <- payload:
		default:
		}
	}
}

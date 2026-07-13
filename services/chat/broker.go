package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const roomChannelPrefix = "chat:room:"

// Broker fans chat messages out across pods over Redis pub/sub: publishing
// a message on one pod delivers it to every pod (including the publisher)
// subscribed to that room, which is what lets stateless pods behind a
// load balancer stay in sync without sharing any local state.
type Broker struct {
	rdb *redis.Client
}

func newBroker(rdb *redis.Client) *Broker {
	return &Broker{rdb: rdb}
}

func roomChannel(room string) string {
	return roomChannelPrefix + room
}

func (b *Broker) publish(ctx context.Context, room string, payload []byte) error {
	return b.rdb.Publish(ctx, roomChannel(room), payload).Err()
}

// subscribe listens on room's Redis channel and invokes onMessage for every
// message received, until ctx is cancelled. It returns immediately; the
// listening happens in a background goroutine.
func (b *Broker) subscribe(ctx context.Context, room string, onMessage func([]byte)) {
	pubsub := b.rdb.Subscribe(ctx, roomChannel(room))

	go func() {
		defer func() { _ = pubsub.Close() }()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				onMessage([]byte(msg.Payload))
			}
		}
	}()
}

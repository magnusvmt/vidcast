package main

import (
	"context"
	"hash/fnv"

	"github.com/redis/go-redis/v9"
)

const roomChannelPrefix = "chat:room:"

// Broker fans chat messages out across pods over Redis pub/sub: publishing
// a message on one pod delivers it to every pod (including the publisher)
// subscribed to that room, which is what lets stateless pods behind a
// load balancer stay in sync without sharing any local state.
//
// A room is deterministically routed to one of possibly several Redis
// shards (see shardIndex), so pub/sub load for a large number of rooms can
// be spread across multiple Redis instances instead of a single one having
// to carry every channel.
type Broker struct {
	shards []*redis.Client
}

func newBroker(shards ...*redis.Client) *Broker {
	return &Broker{shards: shards}
}

func roomChannel(room string) string {
	return roomChannelPrefix + room
}

// shardIndex deterministically maps a room to one of n shards by hashing
// its name, so every pod routes a given room to the same shard without
// needing to coordinate.
func shardIndex(room string, n int) int {
	if n <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(room))
	return int(h.Sum32() % uint32(n))
}

func (b *Broker) shardFor(room string) *redis.Client {
	return b.shards[shardIndex(room, len(b.shards))]
}

func (b *Broker) publish(ctx context.Context, room string, payload []byte) error {
	return b.shardFor(room).Publish(ctx, roomChannel(room), payload).Err()
}

// subscribe listens on room's Redis channel and invokes onMessage for every
// message received, until ctx is cancelled. It returns immediately; the
// listening happens in a background goroutine.
func (b *Broker) subscribe(ctx context.Context, room string, onMessage func([]byte)) {
	pubsub := b.shardFor(room).Subscribe(ctx, roomChannel(room))

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

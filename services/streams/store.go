package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	// ErrChannelExists is returned when creating a stream key for a channel that already has one.
	ErrChannelExists = errors.New("channel already has a stream key")
	// ErrChannelNotFound is returned when operating on a channel that has no provisioned stream key.
	ErrChannelNotFound = errors.New("channel not found")
)

// Channel is the public (non-secret) view of a channel's state.
type Channel struct {
	Slug   string
	HasKey bool
	Live   bool
}

// LiveChannel describes a channel that is currently live.
type LiveChannel struct {
	Slug string `json:"slug"`
	Live bool   `json:"live"`
}

type channelRecord struct {
	slug    string
	keyHash string
	live    bool
}

// store is an in-memory, concurrency-safe registry of channel stream keys and
// live state. Streams don't outlive a running process today: acceptable
// because a restart simply requires streamers to reconnect (MediaMTX itself
// holds no state either), and it keeps this service free of any external
// dependency to stand up.
type store struct {
	mu sync.RWMutex
	// channels indexes records by slug.
	channels map[string]*channelRecord
	// byKeyHash indexes slugs by the sha256 hash of their current stream key,
	// so the auth webhook can resolve a presented key without scanning every
	// channel or storing the key itself in plaintext.
	byKeyHash map[string]string
}

func newStore() *store {
	return &store{
		channels:  make(map[string]*channelRecord),
		byKeyHash: make(map[string]string),
	}
}

// CreateKey provisions a new stream key for slug. It fails with
// ErrChannelExists if the channel already has one; use RotateKey to replace
// an existing key.
func (s *store) CreateKey(slug string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.channels[slug]; exists {
		return "", ErrChannelExists
	}

	key, hash, err := generateKey()
	if err != nil {
		return "", err
	}
	s.channels[slug] = &channelRecord{slug: slug, keyHash: hash}
	s.byKeyHash[hash] = slug
	return key, nil
}

// RotateKey replaces slug's stream key with a freshly generated one,
// invalidating the previous key. It fails with ErrChannelNotFound if the
// channel has no key to rotate.
func (s *store) RotateKey(slug string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, exists := s.channels[slug]
	if !exists {
		return "", ErrChannelNotFound
	}

	key, hash, err := generateKey()
	if err != nil {
		return "", err
	}
	delete(s.byKeyHash, rec.keyHash)
	rec.keyHash = hash
	s.byKeyHash[hash] = slug
	return key, nil
}

// RevokeKey deletes slug's channel entirely, invalidating its key and taking
// it off any live listing. It fails with ErrChannelNotFound if unknown.
func (s *store) RevokeKey(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, exists := s.channels[slug]
	if !exists {
		return ErrChannelNotFound
	}
	delete(s.byKeyHash, rec.keyHash)
	delete(s.channels, slug)
	return nil
}

// Get returns the public state of slug's channel, without its secret key.
func (s *store) Get(slug string) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, exists := s.channels[slug]
	if !exists {
		return Channel{}, false
	}
	return Channel{Slug: rec.slug, HasKey: rec.keyHash != "", Live: rec.live}, true
}

// FindByKey resolves a presented stream key to its channel's slug. Used by
// the MediaMTX auth webhook to authorize publish attempts.
func (s *store) FindByKey(key string) (string, bool) {
	hash := hashKey(key)

	s.mu.RLock()
	defer s.mu.RUnlock()

	slug, ok := s.byKeyHash[hash]
	return slug, ok
}

// SetLive marks slug's channel as live or offline. It fails with
// ErrChannelNotFound if unknown.
func (s *store) SetLive(slug string, live bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, exists := s.channels[slug]
	if !exists {
		return ErrChannelNotFound
	}
	rec.live = live
	return nil
}

// ListLive returns every currently-live channel, sorted by slug.
func (s *store) ListLive() []LiveChannel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	live := make([]LiveChannel, 0)
	for _, rec := range s.channels {
		if rec.live {
			live = append(live, LiveChannel{Slug: rec.slug, Live: true})
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].Slug < live[j].Slug })
	return live
}

func generateKey() (key, hash string, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate stream key: %w", err)
	}
	key = "sk_" + hex.EncodeToString(raw)
	return key, hashKey(key), nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

package main

import (
	"errors"
	"testing"
)

func TestStore_CreateKey_ReturnsUsableKeyAndRejectsDuplicateCreate(t *testing.T) {
	s := newStore()

	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v, want nil", err)
	}
	if key == "" {
		t.Fatal("CreateKey() returned empty key")
	}

	if _, err := s.CreateKey("alice"); !errors.Is(err, ErrChannelExists) {
		t.Fatalf("second CreateKey() error = %v, want ErrChannelExists", err)
	}
}

func TestStore_CreateKey_GeneratesUniqueKeysAcrossChannels(t *testing.T) {
	s := newStore()

	key1, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey(alice) error = %v", err)
	}
	key2, err := s.CreateKey("bob")
	if err != nil {
		t.Fatalf("CreateKey(bob) error = %v", err)
	}

	if key1 == key2 {
		t.Fatalf("CreateKey() produced identical keys for different channels: %q", key1)
	}
}

func TestStore_FindByKey_ResolvesChannelForValidKeyOnly(t *testing.T) {
	s := newStore()
	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	slug, ok := s.FindByKey(key)
	if !ok || slug != "alice" {
		t.Fatalf("FindByKey(valid) = (%q, %v), want (\"alice\", true)", slug, ok)
	}

	if _, ok := s.FindByKey("not-a-real-key"); ok {
		t.Fatal("FindByKey(bogus) = ok, want not found")
	}
}

func TestStore_RotateKey_InvalidatesOldKeyAndIssuesNewOne(t *testing.T) {
	s := newStore()
	oldKey, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	newKey, err := s.RotateKey("alice")
	if err != nil {
		t.Fatalf("RotateKey() error = %v, want nil", err)
	}
	if newKey == oldKey {
		t.Fatal("RotateKey() returned the same key")
	}

	if _, ok := s.FindByKey(oldKey); ok {
		t.Fatal("FindByKey(oldKey) = ok after rotation, want not found")
	}
	if slug, ok := s.FindByKey(newKey); !ok || slug != "alice" {
		t.Fatalf("FindByKey(newKey) = (%q, %v), want (\"alice\", true)", slug, ok)
	}
}

func TestStore_RotateKey_UnknownChannelReturnsNotFound(t *testing.T) {
	s := newStore()

	if _, err := s.RotateKey("nobody"); !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("RotateKey(unknown) error = %v, want ErrChannelNotFound", err)
	}
}

func TestStore_RevokeKey_RemovesChannelAndItsKeyLookup(t *testing.T) {
	s := newStore()
	key, err := s.CreateKey("alice")
	if err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	if err := s.RevokeKey("alice"); err != nil {
		t.Fatalf("RevokeKey() error = %v, want nil", err)
	}

	if _, ok := s.FindByKey(key); ok {
		t.Fatal("FindByKey(key) = ok after revoke, want not found")
	}
	if _, ok := s.Get("alice"); ok {
		t.Fatal("Get(alice) = ok after revoke, want not found")
	}
}

func TestStore_RevokeKey_UnknownChannelReturnsNotFound(t *testing.T) {
	s := newStore()

	if err := s.RevokeKey("nobody"); !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("RevokeKey(unknown) error = %v, want ErrChannelNotFound", err)
	}
}

func TestStore_Get_ReportsWhetherKeyIsProvisionedWithoutLeakingSecret(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	ch, ok := s.Get("alice")
	if !ok {
		t.Fatal("Get(alice) = not found, want found")
	}
	if !ch.HasKey {
		t.Error("Get(alice).HasKey = false, want true")
	}
	if ch.Live {
		t.Error("Get(alice).Live = true, want false before any publish")
	}
}

func TestStore_SetLive_TogglesLiveStateForKnownChannel(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}

	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive(true) error = %v, want nil", err)
	}
	if ch, _ := s.Get("alice"); !ch.Live {
		t.Fatal("channel not live after SetLive(true)")
	}

	if err := s.SetLive("alice", false); err != nil {
		t.Fatalf("SetLive(false) error = %v, want nil", err)
	}
	if ch, _ := s.Get("alice"); ch.Live {
		t.Fatal("channel still live after SetLive(false)")
	}
}

func TestStore_SetLive_UnknownChannelReturnsNotFound(t *testing.T) {
	s := newStore()

	if err := s.SetLive("nobody", true); !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("SetLive(unknown) error = %v, want ErrChannelNotFound", err)
	}
}

func TestStore_RevokeKey_TakesChannelOffAir(t *testing.T) {
	s := newStore()
	if _, err := s.CreateKey("alice"); err != nil {
		t.Fatalf("CreateKey() error = %v", err)
	}
	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive() error = %v", err)
	}

	live := s.ListLive()
	if len(live) != 1 || live[0].Slug != "alice" {
		t.Fatalf("ListLive() = %v, want [alice] before revoke", live)
	}

	if err := s.RevokeKey("alice"); err != nil {
		t.Fatalf("RevokeKey() error = %v", err)
	}

	if live := s.ListLive(); len(live) != 0 {
		t.Fatalf("ListLive() = %v, want empty after revoke", live)
	}
}

func TestStore_ListLive_ReturnsOnlyLiveChannelsSortedBySlug(t *testing.T) {
	s := newStore()
	for _, slug := range []string{"carol", "alice", "bob"} {
		if _, err := s.CreateKey(slug); err != nil {
			t.Fatalf("CreateKey(%s) error = %v", slug, err)
		}
	}
	if err := s.SetLive("alice", true); err != nil {
		t.Fatalf("SetLive(alice) error = %v", err)
	}
	if err := s.SetLive("carol", true); err != nil {
		t.Fatalf("SetLive(carol) error = %v", err)
	}
	// bob stays offline.

	live := s.ListLive()
	if len(live) != 2 {
		t.Fatalf("ListLive() returned %d channels, want 2: %v", len(live), live)
	}
	if live[0].Slug != "alice" || live[1].Slug != "carol" {
		t.Fatalf("ListLive() = %v, want [alice carol] in sorted order", live)
	}
}

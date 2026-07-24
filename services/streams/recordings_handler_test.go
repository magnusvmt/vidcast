package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeRecordingsAPI struct {
	objects map[string][]RecordingObject
	listErr error
	presign func(key string) (string, error)
}

func (f *fakeRecordingsAPI) ListRecordings(_ context.Context, slug string) ([]RecordingObject, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.objects[slug], nil
}

func (f *fakeRecordingsAPI) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if f.presign != nil {
		return f.presign(key)
	}
	return "https://minio.example/" + key + "?signed=1", nil
}

func newRecordingsRequest(slug string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/channels/"+slug+"/recordings", nil)
	req.SetPathValue("slug", slug)
	return req
}

func TestListRecordingsHandler(t *testing.T) {
	t.Run("returns recordings with presigned URLs", func(t *testing.T) {
		recordedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
		api := &fakeRecordingsAPI{
			objects: map[string][]RecordingObject{
				"alice": {
					{Key: "recordings/alice/seg1.mp4", SizeBytes: 1024, LastModified: recordedAt},
				},
			},
		}
		handler := newListRecordingsHandler(api)
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest("alice"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		var got struct {
			Recordings []Recording `json:"recordings"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(got.Recordings) != 1 {
			t.Fatalf("got %d recordings, want 1", len(got.Recordings))
		}
		r := got.Recordings[0]
		if r.Key != "recordings/alice/seg1.mp4" {
			t.Errorf("key = %q", r.Key)
		}
		if r.URL != "https://minio.example/recordings/alice/seg1.mp4?signed=1" {
			t.Errorf("url = %q", r.URL)
		}
		if r.SizeBytes != 1024 {
			t.Errorf("sizeBytes = %d, want 1024", r.SizeBytes)
		}
		if !r.RecordedAt.Equal(recordedAt) {
			t.Errorf("recordedAt = %v, want %v", r.RecordedAt, recordedAt)
		}
	})

	t.Run("channel with no recordings returns an empty list, not null", func(t *testing.T) {
		api := &fakeRecordingsAPI{objects: map[string][]RecordingObject{}}
		handler := newListRecordingsHandler(api)
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest("bob"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"recordings":[]`) {
			t.Fatalf("expected an empty array, got body %s", rec.Body.String())
		}
	})

	t.Run("nil api reports 503 rather than panicking", func(t *testing.T) {
		handler := newListRecordingsHandler(nil)
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest("alice"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("list error is reported as 502", func(t *testing.T) {
		api := &fakeRecordingsAPI{listErr: errors.New("s3 is unreachable")}
		handler := newListRecordingsHandler(api)
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest("alice"))

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", rec.Code)
		}
	})

	t.Run("presign error is reported as 502", func(t *testing.T) {
		api := &fakeRecordingsAPI{
			objects: map[string][]RecordingObject{
				"alice": {{Key: "recordings/alice/seg1.mp4"}},
			},
			presign: func(string) (string, error) { return "", errors.New("signing failed") },
		}
		handler := newListRecordingsHandler(api)
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest("alice"))

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", rec.Code)
		}
	})
}

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"alice", false},
		{"alice-channel", false},
		{"alice_channel", false},
		{"../etc", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{"..", true},
		{"", false},
	}
	for _, tc := range tests {
		got := containsPathTraversal(tc.input)
		if got != tc.want {
			t.Errorf("containsPathTraversal(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestListRecordingsHandler_InvalidSlug(t *testing.T) {
	api := &fakeRecordingsAPI{objects: map[string][]RecordingObject{}}
	handler := newListRecordingsHandler(api)

	for _, slug := range []string{"..", "../etc", "foo/bar", "foo\\bar"} {
		rec := httptest.NewRecorder()
		handler(rec, newRecordingsRequest(slug))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("slug %q: status = %d, want 400; body=%s", slug, rec.Code, rec.Body.String())
		}
	}
}

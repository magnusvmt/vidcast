package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// errRecordingsNotConfigured is returned when this deployment has no
// object storage configured - see loadS3Config in s3client.go.
var errRecordingsNotConfigured = errors.New("VOD recording storage is not configured for this deployment")

// RecordingObject is one completed VOD segment as it exists in object
// storage, before being shaped into the handler's public JSON response.
type RecordingObject struct {
	Key          string
	SizeBytes    int64
	LastModified time.Time
}

// Recording is the public JSON view of one VOD asset.
type Recording struct {
	Key        string    `json:"key"`
	URL        string    `json:"url"`
	RecordedAt time.Time `json:"recordedAt"`
	SizeBytes  int64     `json:"sizeBytes"`
}

// recordingsAPI is the object-storage access this handler needs, narrowed
// so tests can substitute a fake instead of a real S3-compatible backend.
type recordingsAPI interface {
	// ListRecordings returns every recorded segment stored for slug,
	// oldest first.
	ListRecordings(ctx context.Context, slug string) ([]RecordingObject, error)
	// PresignGet returns a time-limited URL a viewer can fetch key from
	// directly, without needing the storage backend's own credentials.
	PresignGet(ctx context.Context, key string, expires time.Duration) (string, error)
}

const recordingURLExpiry = 15 * time.Minute

// registerRecordingsRoutes wires the VOD listing endpoint onto mux:
//
//	GET /channels/{slug}/recordings  every recorded segment for slug, with
//	                                 a presigned playback/download URL
//
// api may be nil if this deployment has no object storage configured (see
// main.go), in which case the endpoint reports 503 rather than panicking.
//
// Deliberately unauthenticated, like registerChannelsRoutes' GET /channels:
// VOD playback is meant to be public in the same way a channel's live
// stream is, so this mirrors the public listing pattern rather than the
// stream-key routes' requireAuth (those guard a secret, this doesn't - the
// presigned URLs it hands out are themselves scoped to one object and 15
// minutes). Revisit if VOD is ever meant to be gated per-channel.
func registerRecordingsRoutes(mux *http.ServeMux, api recordingsAPI) {
	mux.HandleFunc("GET /channels/{slug}/recordings", newListRecordingsHandler(api))
}

func newListRecordingsHandler(api recordingsAPI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if api == nil {
			writeError(w, http.StatusServiceUnavailable, errRecordingsNotConfigured)
			return
		}

		slug := r.PathValue("slug")
		if containsPathTraversal(slug) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid slug %q", slug))
			return
		}
		objects, err := api.ListRecordings(r.Context(), slug)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}

		recordings := make([]Recording, 0, len(objects))
		for _, obj := range objects {
			url, err := api.PresignGet(r.Context(), obj.Key, recordingURLExpiry)
			if err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			recordings = append(recordings, Recording{
				Key:        obj.Key,
				URL:        url,
				RecordedAt: obj.LastModified,
				SizeBytes:  obj.SizeBytes,
			})
		}

		writeJSON(w, http.StatusOK, struct {
			Recordings []Recording `json:"recordings"`
		}{Recordings: recordings})
	}
}

func containsPathTraversal(s string) bool {
	return strings.ContainsAny(s, "/\\") || strings.Contains(s, "..")
}

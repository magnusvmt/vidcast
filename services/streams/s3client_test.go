package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadS3Config(t *testing.T) {
	t.Run("not configured when S3_BUCKET is unset", func(t *testing.T) {
		_, ok := loadS3Config(fakeGetenv(map[string]string{}))
		if ok {
			t.Fatal("expected ok=false when S3_BUCKET is unset")
		}
	})

	t.Run("configured with all fields", func(t *testing.T) {
		cfg, ok := loadS3Config(fakeGetenv(map[string]string{
			"S3_ENDPOINT":   "http://minio.apps.svc:9000",
			"S3_BUCKET":     "vod-recordings",
			"S3_ACCESS_KEY": "vidcast",
			"S3_SECRET_KEY": "s3cret",
		}))
		if !ok {
			t.Fatal("expected ok=true")
		}
		want := s3Config{
			Endpoint:     "http://minio.apps.svc:9000",
			Bucket:       "vod-recordings",
			AccessKey:    "vidcast",
			SecretKey:    "s3cret",
			UsePathStyle: true,
		}
		if cfg != want {
			t.Fatalf("got %+v, want %+v", cfg, want)
		}
	})

	t.Run("S3_USE_PATH_STYLE=false disables path-style addressing", func(t *testing.T) {
		cfg, ok := loadS3Config(fakeGetenv(map[string]string{
			"S3_BUCKET":         "vod-recordings",
			"S3_USE_PATH_STYLE": "false",
		}))
		if !ok || cfg.UsePathStyle {
			t.Fatalf("got ok=%v UsePathStyle=%v, want ok=true UsePathStyle=false", ok, cfg.UsePathStyle)
		}
	})
}

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

const listObjectsV2Response = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>vod-recordings</Name>
  <Prefix>recordings/alice/</Prefix>
  <KeyCount>1</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>recordings/alice/2026-07-24_10-00-00-000000.mp4</Key>
    <LastModified>2026-07-24T10:00:10.000Z</LastModified>
    <ETag>&quot;abc123&quot;</ETag>
    <Size>1048576</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>`

// TestS3RecordingsListAndPresign exercises the real SDK client (not a fake
// recordingsAPI) against an httptest server standing in for MinIO's
// ListObjectsV2 response, to catch endpoint/path-style/parsing mistakes a
// hand-rolled fake wouldn't catch.
func TestS3RecordingsListAndPresign(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, listObjectsV2Response)
	}))
	defer server.Close()

	cfg := s3Config{
		Endpoint:     server.URL,
		Bucket:       "vod-recordings",
		AccessKey:    "vidcast",
		SecretKey:    "vidcast-minio-dev",
		UsePathStyle: true,
	}
	recordings := newS3Recordings(cfg)

	objects, err := recordings.ListRecordings(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/vod-recordings" {
		t.Errorf("request path = %q, want /vod-recordings (path-style addressing)", gotPath)
	}
	if !strings.Contains(gotQuery, "prefix=recordings%2Falice%2F") {
		t.Errorf("query = %q, want it to list under prefix recordings/alice/", gotQuery)
	}
	if len(objects) != 1 {
		t.Fatalf("got %d objects, want 1", len(objects))
	}
	obj := objects[0]
	if obj.Key != "recordings/alice/2026-07-24_10-00-00-000000.mp4" {
		t.Errorf("key = %q", obj.Key)
	}
	if obj.SizeBytes != 1048576 {
		t.Errorf("sizeBytes = %d, want 1048576", obj.SizeBytes)
	}
	wantModified := time.Date(2026, 7, 24, 10, 0, 10, 0, time.UTC)
	if !obj.LastModified.Equal(wantModified) {
		t.Errorf("lastModified = %v, want %v", obj.LastModified, wantModified)
	}

	url, err := recordings.PresignGet(context.Background(), obj.Key, 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected presign error: %v", err)
	}
	wantPrefix := server.URL + "/vod-recordings/" + obj.Key
	if !strings.HasPrefix(url, wantPrefix) {
		t.Errorf("presigned url = %q, want prefix %q (path-style)", url, wantPrefix)
	}
	if !strings.Contains(url, "X-Amz-Expires=900") {
		t.Errorf("presigned url = %q, want X-Amz-Expires=900 (15 minutes)", url)
	}
	if !strings.Contains(url, "X-Amz-Signature=") {
		t.Errorf("presigned url = %q, want a signature", url)
	}
}

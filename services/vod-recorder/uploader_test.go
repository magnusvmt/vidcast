package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestObjectKey(t *testing.T) {
	seg := segment{Path: "alice", LocalPath: "/recordings/alice/2026-07-24_10-00-00-000000.mp4"}
	got := objectKey(seg)
	want := "recordings/alice/2026-07-24_10-00-00-000000.mp4"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestContentType(t *testing.T) {
	cases := map[string]string{
		"/x/seg.mp4": "video/mp4",
		"/x/seg.MP4": "video/mp4",
		"/x/seg.ts":  "video/mp2t",
		"/x/seg.bin": "application/octet-stream",
	}
	for path, want := range cases {
		if got := contentType(path); got != want {
			t.Errorf("contentType(%q) = %q, want %q", path, got, want)
		}
	}
}

type fakePutObjectAPI struct {
	input *s3.PutObjectInput
	body  []byte
	err   error
}

func (f *fakePutObjectAPI) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.input = params
	body, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	f.body = body
	return &s3.PutObjectOutput{}, nil
}

func writeTempSegment(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "2026-07-24_10-00-00-000000.mp4")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp segment: %v", err)
	}
	return p
}

func TestUploadSegment(t *testing.T) {
	t.Run("uploads the file under bucket/objectKey with metadata", func(t *testing.T) {
		localPath := writeTempSegment(t, "fake mp4 bytes")
		seg := segment{Path: "alice", LocalPath: localPath, Duration: 10.5}
		api := &fakePutObjectAPI{}

		if err := uploadSegment(context.Background(), api, "vod-recordings", seg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := aws.ToString(api.input.Bucket); got != "vod-recordings" {
			t.Errorf("bucket = %q, want vod-recordings", got)
		}
		if got := aws.ToString(api.input.Key); got != objectKey(seg) {
			t.Errorf("key = %q, want %q", got, objectKey(seg))
		}
		if string(api.body) != "fake mp4 bytes" {
			t.Errorf("body = %q, want %q", api.body, "fake mp4 bytes")
		}
		if got := api.input.Metadata["duration-seconds"]; got != "10.5" {
			t.Errorf("duration-seconds metadata = %q, want 10.5", got)
		}
		if got := aws.ToString(api.input.ContentType); got != "video/mp4" {
			t.Errorf("content type = %q, want video/mp4", got)
		}
	})

	t.Run("missing local file", func(t *testing.T) {
		seg := segment{Path: "alice", LocalPath: "/does/not/exist.mp4", Duration: 1}
		api := &fakePutObjectAPI{}
		if err := uploadSegment(context.Background(), api, "vod-recordings", seg); err == nil {
			t.Fatal("expected error when the local segment file doesn't exist")
		}
	})

	t.Run("S3 API error is wrapped with the destination", func(t *testing.T) {
		localPath := writeTempSegment(t, "x")
		seg := segment{Path: "alice", LocalPath: localPath}
		api := &fakePutObjectAPI{err: errors.New("boom")}

		err := uploadSegment(context.Background(), api, "vod-recordings", seg)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, api.err) {
			t.Fatalf("expected wrapped error to satisfy errors.Is(_, api.err), got %v", err)
		}
	})
}

// TestNewS3ClientPutObjectRoundTrip exercises the real SDK client (not the
// fake interface above) against an httptest server standing in for
// MinIO, to catch endpoint/path-style/credential wiring mistakes that
// fakePutObjectAPI can't - e.g. an unsigned request or a virtual-hosted
// bucket URL when path-style was intended.
func TestNewS3ClientPutObjectRoundTrip(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config{
		Endpoint:     server.URL,
		Bucket:       "vod-recordings",
		AccessKey:    "vidcast",
		SecretKey:    "vidcast-minio-dev",
		UsePathStyle: true,
	}
	client := newS3Client(cfg)
	seg := segment{Path: "alice", LocalPath: writeTempSegment(t, "fake mp4 bytes"), Duration: 3}

	if err := uploadSegment(context.Background(), client, cfg.Bucket, seg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	wantPath := "/vod-recordings/" + objectKey(seg)
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q (path-style addressing)", gotPath, wantPath)
	}
	if gotAuth == "" {
		t.Error("expected a signed Authorization header, got none")
	}
	if string(gotBody) != "fake mp4 bytes" {
		t.Errorf("body = %q, want %q", gotBody, "fake mp4 bytes")
	}
}

package main

import (
	"testing"
)

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfig(t *testing.T) {
	t.Run("all required vars present", func(t *testing.T) {
		cfg, err := loadConfig(fakeGetenv(map[string]string{
			"S3_ENDPOINT":   "http://minio.apps.svc:9000",
			"S3_BUCKET":     "vod-recordings",
			"S3_ACCESS_KEY": "vidcast",
			"S3_SECRET_KEY": "s3cret",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := config{
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
		cfg, err := loadConfig(fakeGetenv(map[string]string{
			"S3_ENDPOINT":       "https://s3.amazonaws.com",
			"S3_BUCKET":         "vod-recordings",
			"S3_ACCESS_KEY":     "vidcast",
			"S3_SECRET_KEY":     "s3cret",
			"S3_USE_PATH_STYLE": "false",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.UsePathStyle {
			t.Fatal("expected UsePathStyle to be false")
		}
	})

	for _, missing := range []string{"S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Run("missing "+missing, func(t *testing.T) {
			values := map[string]string{
				"S3_ENDPOINT":   "http://minio.apps.svc:9000",
				"S3_BUCKET":     "vod-recordings",
				"S3_ACCESS_KEY": "vidcast",
				"S3_SECRET_KEY": "s3cret",
			}
			delete(values, missing)
			if _, err := loadConfig(fakeGetenv(values)); err == nil {
				t.Fatalf("expected error when %s is missing", missing)
			}
		})
	}
}

func TestLoadSegment(t *testing.T) {
	t.Run("valid segment", func(t *testing.T) {
		seg, err := loadSegment(fakeGetenv(map[string]string{
			"MTX_PATH":             "alice",
			"MTX_SEGMENT_PATH":     "/recordings/alice/2026-07-24_10-00-00-000000.mp4",
			"MTX_SEGMENT_DURATION": "10.5",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := segment{
			Path:      "alice",
			LocalPath: "/recordings/alice/2026-07-24_10-00-00-000000.mp4",
			Duration:  10.5,
		}
		if seg != want {
			t.Fatalf("got %+v, want %+v", seg, want)
		}
	})

	t.Run("missing MTX_PATH", func(t *testing.T) {
		_, err := loadSegment(fakeGetenv(map[string]string{
			"MTX_SEGMENT_PATH":     "/recordings/alice/seg.mp4",
			"MTX_SEGMENT_DURATION": "10.5",
		}))
		if err == nil {
			t.Fatal("expected error when MTX_PATH is missing")
		}
	})

	t.Run("missing MTX_SEGMENT_PATH", func(t *testing.T) {
		_, err := loadSegment(fakeGetenv(map[string]string{
			"MTX_PATH":             "alice",
			"MTX_SEGMENT_DURATION": "10.5",
		}))
		if err == nil {
			t.Fatal("expected error when MTX_SEGMENT_PATH is missing")
		}
	})

	t.Run("unparseable MTX_SEGMENT_DURATION", func(t *testing.T) {
		_, err := loadSegment(fakeGetenv(map[string]string{
			"MTX_PATH":             "alice",
			"MTX_SEGMENT_PATH":     "/recordings/alice/seg.mp4",
			"MTX_SEGMENT_DURATION": "not-a-number",
		}))
		if err == nil {
			t.Fatal("expected error for unparseable MTX_SEGMENT_DURATION")
		}
	})
}

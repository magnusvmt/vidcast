package main

import (
	"fmt"
	"strconv"
	"strings"
)

// config holds the S3-compatible object storage settings this uploader
// needs. It's set once on the recording pod's environment (not per
// invocation), unlike segment which MediaMTX sets fresh for every hook run.
type config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	// UsePathStyle is required for MinIO and most other S3-compatible
	// backends, which don't support the virtual-hosted-style bucket
	// addressing (bucket.endpoint.tld) that AWS S3 itself defaults to.
	UsePathStyle bool
}

// loadConfig reads the object storage settings from environment variables,
// via getenv so tests can supply a fake environment instead of the process's
// real one.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		Endpoint:     getenv("S3_ENDPOINT"),
		Bucket:       getenv("S3_BUCKET"),
		AccessKey:    getenv("S3_ACCESS_KEY"),
		SecretKey:    getenv("S3_SECRET_KEY"),
		UsePathStyle: getenv("S3_USE_PATH_STYLE") != "false",
	}
	for name, v := range map[string]string{
		"S3_ENDPOINT":   cfg.Endpoint,
		"S3_BUCKET":     cfg.Bucket,
		"S3_ACCESS_KEY": cfg.AccessKey,
		"S3_SECRET_KEY": cfg.SecretKey,
	} {
		if v == "" {
			return config{}, fmt.Errorf("required environment variable %s is not set", name)
		}
	}
	return cfg, nil
}

// segment describes one completed MediaMTX recording segment. MediaMTX
// execs the runOnRecordSegmentComplete hook directly (not via a shell), so
// these are always present as real process environment variables rather
// than needing shell-side interpolation - see
// https://github.com/bluenviron/mediamtx/blob/main/internal/core/path.go
// (OnSegmentComplete) for the fields it sets.
type segment struct {
	// Path is MTX_PATH, the MediaMTX path name the recording was published
	// to. This chart configures streams to use the password-field stream
	// key convention (see services/streams/README.md), so Path is always
	// the channel's slug - never a key-embedded path segment.
	Path string
	// LocalPath is MTX_SEGMENT_PATH, the absolute local file MediaMTX just
	// finished writing.
	LocalPath string
	// Duration is MTX_SEGMENT_DURATION parsed as seconds.
	Duration float64
}

// loadSegment reads the MTX_* environment variables MediaMTX sets for a
// runOnRecordSegmentComplete invocation, and validates that path is a flat
// slug (no path separators or traversal components) so objectKey can safely
// build an S3 key without risking cross-channel namespace pollution.
func loadSegment(getenv func(string) string) (segment, error) {
	path := getenv("MTX_PATH")
	if path == "" {
		return segment{}, fmt.Errorf("required environment variable MTX_PATH is not set")
	}
	if containsPathTraversal(path) {
		return segment{}, fmt.Errorf("MTX_PATH %q contains path traversal components (/, ..)", path)
	}
	localPath := getenv("MTX_SEGMENT_PATH")
	if localPath == "" {
		return segment{}, fmt.Errorf("required environment variable MTX_SEGMENT_PATH is not set")
	}
	durationStr := getenv("MTX_SEGMENT_DURATION")
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return segment{}, fmt.Errorf("parse MTX_SEGMENT_DURATION %q: %w", durationStr, err)
	}
	return segment{Path: path, LocalPath: localPath, Duration: duration}, nil
}

// containsPathTraversal returns true if s contains characters or components
// that could escape the intended S3 key prefix when joined with path.Join.
func containsPathTraversal(s string) bool {
	return strings.ContainsAny(s, "/\\") || strings.Contains(s, "..")
}

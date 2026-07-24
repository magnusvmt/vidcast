// Command vod-recorder is invoked directly by MediaMTX's
// runOnRecordSegmentComplete hook (one process per completed segment, not a
// long-running server) to upload the segment MediaMTX just finished writing
// to S3-compatible object storage, then remove the local copy.
//
// MediaMTX execs hook commands directly rather than via a shell, so this
// binary is laid down next to the mediamtx binary in the image built by
// this directory's Dockerfile - see deploy/charts/mediamtx's
// runOnRecordSegmentComplete value.
package main

import (
	"context"
	"log"
	"os"
)

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("vod-recorder: %v", err)
	}
	seg, err := loadSegment(os.Getenv)
	if err != nil {
		log.Fatalf("vod-recorder: %v", err)
	}

	client := newS3Client(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.UploadTimeout)
	defer cancel()
	if err := uploadSegment(ctx, client, cfg.Bucket, seg); err != nil {
		log.Fatalf("vod-recorder: %v", err)
	}
	log.Printf("vod-recorder: uploaded %s to s3://%s/%s", seg.LocalPath, cfg.Bucket, objectKey(seg))

	// Best-effort: recordDeleteAfter (set in the mediamtx chart's values)
	// is the backstop if this fails, so a leftover local file isn't fatal.
	if err := os.Remove(seg.LocalPath); err != nil {
		log.Printf("vod-recorder: uploaded but failed to remove local copy %s: %v", seg.LocalPath, err)
	}
}

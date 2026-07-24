package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// staticCredentials is a minimal aws.CredentialsProvider for a fixed
// access/secret key pair. Written by hand rather than pulling in the
// separate aws-sdk-go-v2/credentials module (whose NewStaticCredentialsProvider
// does the same thing) so this tool's dependency graph - and therefore its
// minimum Go version - stays aligned with the rest of the repo's services.
type staticCredentials aws.Credentials

func (c staticCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials(c), nil
}

// newS3Client builds an S3 client pointed at cfg's endpoint, suitable for
// MinIO or any other S3-compatible backend - not just AWS itself.
func newS3Client(cfg config) *s3.Client {
	return s3.New(s3.Options{
		// The SDK requires a non-empty region even against a non-AWS,
		// single-region backend like MinIO; the value is never validated
		// against anything since BaseEndpoint overrides AWS's own
		// region-based endpoint resolution.
		Region:       "us-east-1",
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.UsePathStyle,
		Credentials: staticCredentials{
			AccessKeyID:     cfg.AccessKey,
			SecretAccessKey: cfg.SecretKey,
		},
	})
}

// objectKey derives the bucket key a segment should be stored under: one
// prefix per channel path, named after the segment file MediaMTX already
// wrote (which itself encodes the recording's start time via the
// recordPath template configured in the mediamtx Helm chart's values).
func objectKey(seg segment) string {
	return path.Join("recordings", seg.Path, filepath.Base(seg.LocalPath))
}

// contentType returns the MIME type for a recorded segment, based on the
// extension MediaMTX gave it (see recordstore.PathAddExtension upstream:
// ".mp4" for the fmp4 recordFormat, ".ts" for mpegts).
func contentType(localPath string) string {
	switch strings.ToLower(filepath.Ext(localPath)) {
	case ".mp4":
		return "video/mp4"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}

// putObjectAPI is the one S3 operation uploadSegment needs, narrowed from
// *s3.Client so tests can substitute a fake without a real network call.
type putObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// uploadSegment uploads seg's local file to bucket under objectKey(seg),
// tagging the object with the segment duration as user metadata so the
// streams service can surface it without re-probing the file.
func uploadSegment(ctx context.Context, api putObjectAPI, bucket string, seg segment) error {
	f, err := os.Open(seg.LocalPath)
	if err != nil {
		return fmt.Errorf("open segment file %s: %w", seg.LocalPath, err)
	}
	defer f.Close()

	key := objectKey(seg)
	_, err = api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType(seg.LocalPath)),
		Metadata: map[string]string{
			"duration-seconds": strconv.FormatFloat(seg.Duration, 'f', -1, 64),
		},
	})
	if err != nil {
		return fmt.Errorf("upload segment to s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

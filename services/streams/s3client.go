package main

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3Config holds the object storage settings the recordings-listing
// endpoint needs. Recording itself (uploading segments) is done by the
// separate vod-recorder tool that runs inside the mediamtx pod; this
// service only ever reads the bucket back out, to list and present VOD
// assets.
type s3Config struct {
	Endpoint     string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// loadS3Config reads recording storage settings from environment
// variables. It returns ok=false if S3_BUCKET is unset, since VOD listing
// is an optional feature - a deployment with no object storage configured
// should still start up and serve its other endpoints.
func loadS3Config(getenv func(string) string) (cfg s3Config, ok bool) {
	bucket := getenv("S3_BUCKET")
	if bucket == "" {
		return s3Config{}, false
	}
	return s3Config{
		Endpoint:     getenv("S3_ENDPOINT"),
		Bucket:       bucket,
		AccessKey:    getenv("S3_ACCESS_KEY"),
		SecretKey:    getenv("S3_SECRET_KEY"),
		UsePathStyle: getenv("S3_USE_PATH_STYLE") != "false",
	}, true
}

// staticCredentials is a minimal aws.CredentialsProvider for a fixed
// access/secret key pair, written by hand rather than pulling in the
// separate aws-sdk-go-v2/credentials module so this service's dependency
// graph - and therefore its minimum Go version - doesn't move independently
// of the rest of the repo's services.
type staticCredentials aws.Credentials

func (c staticCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials(c), nil
}

// s3Recordings implements recordingsAPI against a real S3-compatible
// bucket (MinIO or otherwise).
type s3Recordings struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func newS3Recordings(cfg s3Config) *s3Recordings {
	client := s3.New(s3.Options{
		// Required by the SDK even against a non-AWS, single-region
		// backend like MinIO; BaseEndpoint overrides AWS's own
		// region-based endpoint resolution so the value itself is never
		// validated against anything.
		Region:       "us-east-1",
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.UsePathStyle,
		Credentials: staticCredentials{
			AccessKeyID:     cfg.AccessKey,
			SecretAccessKey: cfg.SecretKey,
		},
	})
	return &s3Recordings{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
	}
}

// ListRecordings lists every object vod-recorder has uploaded for slug -
// see objectKey in services/vod-recorder/uploader.go for the matching key
// scheme this depends on (recordings/<slug>/<segment file>).
func (s *s3Recordings) ListRecordings(ctx context.Context, slug string) ([]RecordingObject, error) {
	prefix := path.Join("recordings", slug) + "/"
	var objects []RecordingObject
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list recordings under s3://%s/%s: %w", s.bucket, prefix, err)
		}
		for _, obj := range page.Contents {
			objects = append(objects, RecordingObject{
				Key:          aws.ToString(obj.Key),
				SizeBytes:    aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}
	}
	return objects, nil
}

// PresignGet returns a time-limited GET URL for key, so a viewer can
// stream/download it directly from the bucket without needing this
// service's own storage credentials.
func (s *s3Recordings) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("presign GET for s3://%s/%s: %w", s.bucket, key, err)
	}
	return req.URL, nil
}

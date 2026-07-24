# vod-recorder

Not a long-running service: this is a small CLI, one process per invocation,
that MediaMTX execs directly as its `runOnRecordSegmentComplete` hook to
upload a just-finished recording segment to S3-compatible object storage
(MinIO in this repo), then delete the local copy.

MediaMTX has no native S3 upload support, and shell-tokenizes/execs hook
commands itself rather than invoking a shell (see
[cmd_os.go](https://github.com/bluenviron/mediamtx/blob/main/internal/externalcmd/cmd_os.go)),
so this binary is laid down next to the official `mediamtx` binary by this
directory's `Dockerfile` and pointed to directly - no shell required in the
image at all. See `deploy/charts/mediamtx`'s `recording` values for how the
two are wired together (recordPath, recordFormat, the hook command, and the
S3 env vars below).

## Environment variables

Set once on the mediamtx pod (not per invocation):

| Variable            | Purpose                                                    |
|----------------------|-------------------------------------------------------------|
| `S3_ENDPOINT`        | Object storage endpoint, e.g. `http://minio.apps.svc.cluster.local:9000` |
| `S3_BUCKET`          | Bucket to upload into                                      |
| `S3_ACCESS_KEY`      | Access key                                                 |
| `S3_SECRET_KEY`      | Secret key                                                 |
| `S3_USE_PATH_STYLE`  | `"false"` to use virtual-hosted-style addressing instead of path-style (default: path-style, required by MinIO) |
| `S3_UPLOAD_TIMEOUT`  | Go duration (e.g. `"45m"`) bounding the segment upload; default `30m`. Scale this with `recording.segmentDuration`/bitrate - a segment can be several GB |

MediaMTX sets these fresh for every hook invocation (see
[path.go's OnSegmentComplete](https://github.com/bluenviron/mediamtx/blob/main/internal/core/path.go)):

| Variable               | Meaning                                              |
|-------------------------|-------------------------------------------------------|
| `MTX_PATH`              | The MediaMTX path name the recording was published to |
| `MTX_SEGMENT_PATH`      | Absolute local path of the completed segment file      |
| `MTX_SEGMENT_DURATION`  | Segment duration in seconds (e.g. `"10.5"`)            |

## Object key scheme

Uploaded to `recordings/<MTX_PATH>/<basename of MTX_SEGMENT_PATH>` - see
`objectKey` in `uploader.go`. The segment filename already encodes its start
time via the `recordPath` template configured in the mediamtx chart, so no
extra timestamp bookkeeping happens here. The `services/streams` service
lists these back out under `GET /channels/{slug}/recordings` (see its
README for the `path == slug` assumption this depends on).

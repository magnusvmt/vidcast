# streams

Stream-key CRUD, MediaMTX external-auth webhook, live-channel listing, and
VOD recording listing. Channel/key state is in-memory only (see the comment
on `store` in `store.go` for why that's an acceptable tradeoff for now); VOD
asset metadata lives entirely in object storage instead (see below), so it
isn't lost on a restart even though channel state is.

## API

| Method | Path                          | Purpose                                    |
|--------|-------------------------------|---------------------------------------------|
| POST   | `/channels/{slug}/stream-key` | Create a stream key (409 if one exists)   |
| GET    | `/channels/{slug}/stream-key` | Read metadata — never the secret itself   |
| PUT    | `/channels/{slug}/stream-key` | Rotate the key, invalidating the old one  |
| DELETE | `/channels/{slug}/stream-key` | Revoke the channel and its key            |
| GET    | `/channels`                   | List channels currently live               |
| GET    | `/channels/{slug}/recordings` | List slug's VOD recordings with presigned URLs (503 if no object storage is configured) |
| POST   | `/mediamtx/auth`              | MediaMTX external-auth webhook             |
| POST   | `/mediamtx/unpublish`         | MediaMTX `runOnUnpublish` hook target      |

## VOD recordings (`/channels/{slug}/recordings`)

This service never uploads recordings itself - that's done by the separate
`services/vod-recorder` tool, which MediaMTX execs directly as its
`runOnRecordSegmentComplete` hook (see that service's README and the
`deploy/charts/mediamtx` chart's `recording` values). This endpoint only
*reads the bucket back*: it lists objects under `recordings/{slug}/` and
returns each with a 15-minute presigned GET URL, so a viewer can
stream/download a VOD without needing this service's own storage
credentials.

Configured via the same `S3_ENDPOINT`/`S3_BUCKET`/`S3_ACCESS_KEY`/
`S3_SECRET_KEY`/`S3_USE_PATH_STYLE` environment variables as vod-recorder
(see its README), pointed at the same bucket. If `S3_BUCKET` is unset, this
service still starts up and serves everything else - the recordings
endpoint just reports 503 instead.

Recording keys objects by the raw MediaMTX path name a stream was published
to, not by a resolved channel slug - vod-recorder runs in MediaMTX's pod and
has no way to ask this service to resolve one. This lines up as long as the
deployed MediaMTX config uses the password-field stream-key convention
(`path == slug`, see "Wiring into MediaMTX" below) rather than the
path-embedded-key convention; the latter would list recordings under a
noisy, key-bearing path instead of the plain slug.

## Wiring into MediaMTX (not done by this service)

This service only exposes the HTTP endpoints above; pointing a running
MediaMTX instance at them is deploy/config work for whichever issue wires up
the streams Helm chart. Two things that config needs to get right:

- `authHTTPAddress` → `http://<this-service>/mediamtx/auth`. MediaMTX POSTs a
  JSON body (`action`, `path`, `password`, plus fields this service ignores)
  for every publish/read/etc. request; a non-2xx `action: publish` here also
  works as "reject the key". Reads are always authorized — this platform has
  no private-channel concept yet.
- `runOnUnpublish` → a command that POSTs `{"path": "%path%"}` to
  `/mediamtx/unpublish`, so a channel is marked offline when its source
  disconnects (the auth webhook only fires on connect, never on disconnect).

The stream key itself can be presented either way, and both are accepted so
the config author can pick whichever fits the chosen RTMP URL shape:

- as the webhook's `password` field (e.g. a custom RTMP URL of the form
  `rtmp://x:<key>@host/{slug}`), or
- as the trailing segment of `path` (e.g. Server `rtmp://host/live`, Stream
  Key `<key>`, which OBS concatenates into path `live/<key>`).

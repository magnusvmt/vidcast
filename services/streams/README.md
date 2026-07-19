# streams

Stream-key CRUD, MediaMTX external-auth webhook, and live-channel listing.
State is in-memory only (see the comment on `store` in `store.go` for why
that's an acceptable tradeoff for now).

## API

| Method | Path                        | Purpose                                    |
|--------|-----------------------------|---------------------------------------------|
| POST   | `/channels/{slug}/stream-key` | Create a stream key (409 if one exists)   |
| GET    | `/channels/{slug}/stream-key` | Read metadata — never the secret itself   |
| PUT    | `/channels/{slug}/stream-key` | Rotate the key, invalidating the old one  |
| DELETE | `/channels/{slug}/stream-key` | Revoke the channel and its key            |
| GET    | `/channels`                 | List channels currently live               |
| POST   | `/mediamtx/auth`            | MediaMTX external-auth webhook             |
| POST   | `/mediamtx/unpublish`       | MediaMTX `runOnUnpublish` hook target      |

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

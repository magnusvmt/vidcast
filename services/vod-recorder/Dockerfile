FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/vod-recorder .

# Not a standalone image: this is the official off-the-shelf MediaMTX image
# (see deploy/charts/mediamtx/values.yaml for why that chart doesn't use
# vidcast-lib) with vod-recorder laid down alongside it. MediaMTX execs
# runOnRecordSegmentComplete directly rather than via a shell (it
# shell-tokenizes the config string itself - see
# https://github.com/bluenviron/mediamtx/blob/main/internal/externalcmd/cmd_os.go),
# so the hook command can point straight at /usr/local/bin/vod-recorder
# without needing a shell in this scratch-based image at all.
FROM bluenviron/mediamtx:1.19.2
COPY --from=build /out/vod-recorder /usr/local/bin/vod-recorder

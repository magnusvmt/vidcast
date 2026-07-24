{{- define "mediamtx.labels" -}}
app.kubernetes.io/name: mediamtx
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "mediamtx.configData" -}}
rtsp: false
rtmp: true
rtmpAddress: :{{ .Values.rtmp.port }}
hls: true
hlsAddress: :{{ .Values.hls.port }}
webrtc: false
srt: false
moq: false
{{- if .Values.recording.enabled }}
record: true
recordPath: /recordings/%path/%Y-%m-%d_%H-%M-%S-%f
recordFormat: fmp4
recordSegmentDuration: {{ .Values.recording.segmentDuration }}
recordDeleteAfter: {{ .Values.recording.deleteAfter }}
# vod-recorder (baked into this chart's image - see values.yaml's image
# comment) uploads the completed segment to S3-compatible object storage
# and removes the local copy; recordDeleteAfter above is just the backstop.
# MediaMTX execs this directly (no shell), reading its own S3_* config from
# this pod's environment (set in deployment.yaml) and the MTX_* segment
# details this hook fires with.
runOnRecordSegmentComplete: /usr/local/bin/vod-recorder
{{- end }}
{{- end -}}

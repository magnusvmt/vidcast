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
{{- end -}}

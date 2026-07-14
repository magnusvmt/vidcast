{{- define "mediamtx.labels" -}}
app.kubernetes.io/name: mediamtx
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

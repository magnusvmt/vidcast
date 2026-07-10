{{- define "vidcast-lib.name" -}}
{{- .Chart.Name -}}
{{- end -}}

{{- define "vidcast-lib.fullname" -}}
{{- .Release.Name -}}
{{- end -}}

{{- define "vidcast-lib.labels" -}}
app.kubernetes.io/name: {{ include "vidcast-lib.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "vidcast-lib.service" -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "vidcast-lib.fullname" . }}
  labels:
    {{- include "vidcast-lib.labels" . | nindent 4 }}
spec:
  selector:
    {{- include "vidcast-lib.labels" . | nindent 4 }}
  ports:
    - name: http
      port: {{ .Values.service.port }}
      targetPort: {{ .Values.service.targetPort }}
{{- end -}}

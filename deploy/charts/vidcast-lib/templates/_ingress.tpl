{{- define "vidcast-lib.ingress" -}}
{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "vidcast-lib.fullname" . }}
spec:
  rules:
    - host: {{ .Values.ingress.host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "vidcast-lib.fullname" . }}
                port:
                  number: {{ .Values.service.port }}
{{- end -}}
{{- end -}}

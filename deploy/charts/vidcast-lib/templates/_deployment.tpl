{{- define "vidcast-lib.deployment" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "vidcast-lib.fullname" . }}
  labels:
    {{- include "vidcast-lib.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "vidcast-lib.labels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "vidcast-lib.labels" . | nindent 8 }}
    spec:
      containers:
        - name: {{ include "vidcast-lib.name" . }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.targetPort }}
          {{- with .Values.env }}
          env:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          readinessProbe:
            httpGet:
              path: /
              port: http
          livenessProbe:
            httpGet:
              path: /
              port: http
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          securityContext:
            runAsNonRoot: true
            runAsUser: {{ .Values.securityContext.runAsUser | default 65532 }}
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
{{- end -}}

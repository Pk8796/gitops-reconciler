{{- define "app.labels" -}}
app.kubernetes.io/part-of: gitops-reconciler
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

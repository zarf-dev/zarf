{{/*
Expand the name of the chart.
*/}}
{{- define "simple-helm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

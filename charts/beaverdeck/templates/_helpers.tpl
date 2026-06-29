{{- define "beaverdeck.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "beaverdeck.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "beaverdeck.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride -}}
{{- end -}}

{{- define "beaverdeck.labels" -}}
app.kubernetes.io/name: {{ include "beaverdeck.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "beaverdeck.selectorLabels" -}}
app.kubernetes.io/name: {{ include "beaverdeck.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "beaverdeck.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "beaverdeck.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "beaverdeck.clusterRoleName" -}}
{{- default (printf "%s-operator" (include "beaverdeck.fullname" .)) .Values.rbac.clusterRoleName -}}
{{- end -}}

{{- define "beaverdeck.suppressedInsightsConfigMapName" -}}
{{- default (printf "%s-suppressed-insights" (include "beaverdeck.fullname" .)) .Values.suppressedInsights.configMapName | trunc 63 | trimSuffix "-" -}}
{{- end -}}

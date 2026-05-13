{{/*
Expand the name of the chart.
*/}}
{{- define "app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name. Truncated at 63 chars (DNS-1123).
*/}}
{{- define "app.fullname" -}}
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

{{/*
Chart label value: name-version sanitized.
*/}}
{{- define "app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every rendered resource.
*/}}
{{- define "app.labels" -}}
helm.sh/chart: {{ include "app.chart" . }}
{{ include "app.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels (stable subset of common labels).
*/}}
{{- define "app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Service account name.
*/}}
{{- define "app.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "app.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
ConfigMap name.
*/}}
{{- define "app.configMapName" -}}
{{- printf "%s-config" (include "app.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Gateway name (defaults to fullname).
*/}}
{{- define "app.gatewayName" -}}
{{- default (include "app.fullname" .) .Values.gateway.name -}}
{{- end -}}

{{/*
HTTPRoute name (defaults to fullname).
*/}}
{{- define "app.httpRouteName" -}}
{{- default (include "app.fullname" .) .Values.httpRoute.name -}}
{{- end -}}

{{/*
OnePasswordItem / generated Secret name for a given item entry.
Usage: include "app.onePasswordItemName" (dict "root" $ "item" $entry)
*/}}
{{- define "app.onePasswordItemName" -}}
{{- $root := .root -}}
{{- $item := .item -}}
{{- printf "%s-%s" (include "app.fullname" $root) $item.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Render envFrom entries: ConfigMap (if enabled with data) + each
OnePasswordItem-backed Secret with mountAsEnvFrom != false. User-supplied
container.envFrom is appended verbatim by the caller.
*/}}
{{- define "app.envFrom" -}}
{{- if and .Values.configMap.enabled .Values.configMap.data }}
- configMapRef:
    name: {{ include "app.configMapName" . }}
{{- end }}
{{- if .Values.onePassword.enabled }}
{{- range .Values.onePassword.items }}
{{- if ne .mountAsEnvFrom false }}
- secretRef:
    name: {{ include "app.onePasswordItemName" (dict "root" $ "item" .) }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Values.container.envFrom }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end -}}

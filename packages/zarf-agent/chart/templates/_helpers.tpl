{{/* vim: set filetype=mustache: */}}

{{/*
Generate selector expression based on mode.
Active mode: exclude resources with zarf.dev/agent: skip|ignore
Passive mode: only include resources with zarf.dev/agent: mutate, exclude resources with zarf.dev/agent: skip|ignore
Usage: {{ include "zarf-agent.webhook.selectorExpression" "active" }}
*/}}
{{- define "zarf-agent.webhook.selectorExpression" -}}
- key: zarf.dev/agent
  operator: NotIn
  values:
    - "skip"
    - "ignore"
{{- if eq . "passive" }}
- key: zarf.dev/agent
  operator: In
  values:
    - "mutate"
{{- end -}}
{{- end -}}

{{/* Namespace selector expression - passes namespace mode to selectorExpression */}}
{{- define "zarf-agent.webhook.namespaceSelectorExpression" -}}
{{- include "zarf-agent.webhook.selectorExpression" .Values.mode.namespaces -}}
{{- end -}}

{{/* Object selector expression - passes object mode to selectorExpression */}}
{{- define "zarf-agent.webhook.objectSelectorExpression" -}}
{{- include "zarf-agent.webhook.selectorExpression" .Values.mode.objects -}}
{{- end -}}

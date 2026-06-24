{{- define "zarf-agent.agentIgnoreExpr" -}}
- key: zarf.dev/agent
  operator: NotIn
  values:
    - "skip"
    - "ignore"
{{- end }}

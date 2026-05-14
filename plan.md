# Plan: Agent Opt-In Mutation Mode

Add a `--agent-opt-in-mutations` flag to `zarf init`. When set, the agent only mutates resources that have explicitly opted in via `zarf.dev/agent: mutate` on the resource or its namespace. The current opt-out behavior is preserved as the default.

Routing decisions move entirely into Go handler code — the webhook selectors lose their `ignore` filters and only retain hard safety exclusions (kube-system, K3s Klipper).

## Decision logic

Resource annotation takes priority. If unset, namespace annotation decides. If neither is set, the mode determines the default.

| Resource annotation | Namespace annotation | Opt-out result | Opt-in result |
|---------------------|----------------------|----------------|---------------|
| `mutate`            | any                  | mutate         | mutate        |
| `ignore`            | any                  | skip           | skip          |
| (unset)             | `mutate`             | mutate         | mutate        |
| (unset)             | `ignore`             | skip           | skip          |
| (unset)             | (unset)              | mutate         | skip          |

## Steps

### 1. Extend `State` and `MergeOptions` — `src/pkg/state/state.go`

Add `AgentMutationMode string` to `State` (alongside `AgentTLSUserProvided`, line ~155) and to `MergeOptions` (alongside `AgentTLS`, line ~477). Wire it through `Merge()` inside the `opts.Services.Has(AgentKey)` block. Default value is `"opt-in"`.

`MutationMode` is kept as a typed string in the agent package (`src/internal/agent/http/admission/mutate.go`) to avoid coupling the state package to agent internals. `State` stores a plain string; the agent converts it on startup.

### 2. Add init flag — `src/cmd/initialize.go`

Add `--agent-mutation-mode` string flag with default `"opt-in"`. Validate the value is one of `opt-in` or `opt-out` before calling `Merge`. Pass the value into `MergeOptions.AgentMutationMode`.

### 3. Add template variable — `src/internal/packager/template/template.go`

In the `case "zarf-agent":` block (line ~76), add:

```go
builtinMap["AGENT_MUTATION_MODE"] = s.AgentMutationMode
```

This produces `###ZARF_AGENT_MUTATION_MODE###` following the existing pattern.

### 4. Thread mode through the Helm chart

**`packages/zarf-agent/chart/values.yaml`** — add:
```yaml
mutationMode: "###ZARF_AGENT_MUTATION_MODE###"
```

**`packages/zarf-agent/chart/templates/deployment.yaml`** — add an env var to the `server` container:
```yaml
env:
  - name: ZARF_AGENT_MUTATION_MODE
    value: {{ .Values.mutationMode }}
```

### 5. Conditionalize namespace ignore filter — `packages/zarf-agent/chart/templates/webhook.yaml`

Use a Helm conditional on `namespaceSelector` to include the `zarf.dev/agent: NotIn [skip, ignore]` expression only in opt-out mode:

```yaml
{{- if eq .Values.mutationMode "opt-out" }}
- key: zarf.dev/agent
  operator: NotIn
  values:
    - "skip"
    - "ignore"
{{- end }}
```

In opt-out mode the namespace-level filter stays, preserving the current behavior and avoiding unnecessary handler invocations. In opt-in mode it is removed, which is required because a resource with `mutate` inside an `ignore` namespace should still be mutated — the selector would otherwise block the webhook from firing at all.

Apply this to all 8 webhooks. Keep the `zarf.dev/agent: NotIn [skip, ignore]` expression on `objectSelector` unconditionally — resource `ignore` means skip in every mode, so filtering at the API server level is always safe. Also keep the `kube-system` exclusion and the K3s Klipper label exclusion.

### 6. Add namespace RBAC — `packages/zarf-agent/chart/templates/clusterrole.yaml`

Add a rule for namespaces alongside the existing services rule:
```yaml
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get"]
```

### 7. Label Zarf-managed namespaces — `src/pkg/cluster/namespace.go`

Add `zarf.dev/agent: mutate` in `AdoptZarfManagedLabels`. This propagates automatically to both `NewZarfManagedNamespace` and `NewZarfManagedApplyNamespace`, covering all namespace creation paths:

- `src/pkg/cluster/cluster.go:197` — Zarf namespace at init
- `src/pkg/packager/deploy.go:753` — Zarf namespace during deploy
- `src/internal/packager/helm/post-render.go:80, 268` — namespaces discovered during Helm rendering

In opt-in mode, resources in these namespaces are automatically opted in. In opt-out mode, the label is present but the handler treats unset and `mutate` the same way.

### 8. Mutation decision logic — new file `src/internal/agent/http/admission/mutate.go`

```go
type MutationMode string

const (
    MutationModeOptOut MutationMode = "opt-out"
    MutationModeOptIn  MutationMode = "opt-in"
)

// ModeFromEnv reads ZARF_AGENT_MUTATION_MODE, defaults to opt-out.
func ModeFromEnv() MutationMode

// ShouldMutate returns whether the agent should mutate a resource given its
// annotation, the namespace annotation, and the configured mode.
func ShouldMutate(resourceLabels, nsLabels map[string]string, mode MutationMode) bool
```

### 9. Thread mode through `start.go` and hooks — `src/internal/agent/start.go`, `src/internal/agent/hooks/`

- **`start.go`** — call `ModeFromEnv()` once at startup, pass the result to each hook constructor.
- **Each hook** — add a `mode MutationMode` field. At the top of each `Create`/`Update` handler, call `clientset.CoreV1().Namespaces().Get` to fetch namespace labels, then call `ShouldMutate`. If false, return an allow result with no patches.

### 10. Tests (TDD order)

- `mutate_test.go` — table-driven unit tests for `ShouldMutate` covering all annotation combinations × both modes. Write first, watch fail, then implement.
- Update hook tests to pass a mode and a fake clientset with a namespace object.

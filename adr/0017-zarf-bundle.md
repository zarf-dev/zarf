# 17. Bundles

Date: 2023-06-13

## Status

Pending

## Context

Orchestrating Zarf packages into meta-packages is a current weakpoint of Zarf. The core of Zarf was oriented around components as capabilities, but as Zarf packages have scaled, it has been discovered that entire packages are now rapidly becoming modularized capabilities.

Currently there is no official way to enable the deployment, publishing, pulling, and creation of these Zarf packages. Due to this shortfalling, it has led to the community developing such antipatterns as:

```yaml
- name: init
  required: true
  files:
    - source: zarf-init-amd64-v0.27.0.tar.zst
      target: zarf-init-amd64-v0.27.0.tar.zst
  actions:
    onDeploy:
      after:
        - cmd: zarf package deploy zarf-init-amd64-v0.27.0.tar.zst --components git-server --confirm -l warn
```

While this _does_ fulfill the need to deploy two packages in one command, it does so in such a way that is very brittle and creates bloated and inefficient packages.

### Proposed Solutions

There are currently three proposed solutions to this, each with their own pros and cons:

#### Zarf Bundle

Uses OCI and a separate zarf schema / declarative YAML definition to pull packages into a single artifact and orchestrate them together.

Biggest problem to solve is how to handle variables and whether/how those should be shared between packages.

Package sources in bundles would be OCI _only_, and would not support local packages.

#### Super Zarf Packages

Add a packages key or another way to overlay packages into a larger package with the same internal structure.

Biggest problem here is how we would scope things after layering things together such as how we might need to change Helm chart name generation or other package level things (like deployed package secrets)

#### Zarf Package Manager

Have a separate binary pull and manage packages together - this would also likely include dependency declaration and resolution between packages.

Biggest problem is how do we define dependencies and know what is "installed" for a Zarf package.  Variable orchestration would also be an issue. The added workload of supporting an entire separate binary would also place strain on the Zarf team.

## Decision

The current proposition (subject to change before acceptance) is **Zarf Bundles**, which a following PR will focus on and create a POC of.

In essense the `zarf-bundle.yaml` would look something like so:

```yaml
metadata:
  name: omnibus
  description: an example Zarf bundle
  version: 0.0.1
  arch: amd64

packages:
  - repository: ghcr.io/defenseunicorns/packages/dubbd
    ref: 0.0.1 # OCI spec compliant reference
    # arch is not needed as it will use w/e arch is set in the bundle's metadata
    components:
      - "*" # grab all components
  - repository: docker.io/<namespace>/<name>
    ref: 0.0.1
    components:
      - first-component
      - another-component
```

Bundle would be a new top-level command in Zarf with a full compliment of sub-commands (mirroring the pattern of `zarf package`):

- `zarf bundle create <path> -o oci://<reference>|<path>`
- `zarf bundle deploy oci://<reference>|<path>`
- `zarf bundle inspect oci://<reference>|<path>`
- `zarf bundle list` --> will probably just show the same as `zarf package list`
- ~~`zarf bundle publish`~~ --> Bundles will be OCI only, so there is no need for a publish command, `create` will handle that.
- `zarf bundle pull oci://<reference> -o <dir>`
- `zarf bundle remove oci://<reference>|<path>`

## Consequences

This does add complexity to the Zarf codebase, as it is the addition of an entire suite of commands, JSON schema, schema docs, CLI docs, and chunk of library code + tests.  It is a good litmus test of the current packager and OCI codebase to see how ready it is to be consumed as a library.

There is also the hard requirement that packages bundled must be first published to a registry available to the person performing the bundle operation. This removes some ability to develop bundles on an air gapped environment, but the team believes that in such scenarios, the air gapped environment should be _receiving_ a bundle, rather than developing one internal.

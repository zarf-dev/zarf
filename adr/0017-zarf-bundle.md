# 17. Bundles

Date: 2023-06-13

## Status

[Migrated](https://github.com/defenseunicorns/uds-cli)

## Context

Orchestrating capabilities from multiple Zarf packages into meta-packages is a current weak point for Zarf. The core of Zarf was built around components as capabilities, but as Zarf packages have scaled, there has been a need to create a new boundary to manage these capabilities efficiently.

Currently there is no official way to enable the deployment, publishing, pulling, and creation of multiple Zarf packages together, and due to this some in the community have resorted to patterns such as:

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

While this _does_ fulfill the need to deploy two packages in one command, it does so in such a way that is verbose within the `zarf.yaml`, brittle across Zarf versions, inefficient within the package structure (it doesn't share layers), and is difficult to use `variables` with.

### Proposed Solutions

There are currently three proposed solutions to this, each with their own pros and cons:

#### Zarf Bundle

Uses OCI and a separate Zarf schema / declarative YAML definition to pull packages into a single artifact and orchestrate them together.

Pros:

- maintains efficient OCI layering / deduping of shared package resources
- allows flexibility in defining what `zarf bundle` (or a separate command) would look like as its own command without polluting `zarf package`

Cons:

- variables set within packages with `setVariables` may be difficult to share across packages
- package sources in bundles would be best to keep as OCI _only_, without support for local packages. This would help us ensure there are versions for packages and would help with efficiency by taking advantage of things like layer deduping.

#### Super Zarf Packages

Adds a packages key or another way to overlay packages into a larger package with the same internal structure as current Zarf packages.

Pros:

- packages would maintain the same syntax under `zarf package` between normal / meta packages.

Cons:

- it would be difficult to properly scope things like variables and helm chart names properly across packages.
- this continues to add to `zarf package` making it more complex and harder to test

#### Zarf Package Manager

Uses a separate binary (not `zarf`) to pull and manage packages together - this would also include dependency declaration and resolution between packages.

Pros:

- this is a familiar/expressive way to solve the package problem and would be familiar to developers and system administrators
- allows flexibility in defining what the package manager would look like as its own command without polluting `zarf package`

Cons:

- dependencies may be difficult to determine whether they are "installed"/"deployed" - particularly for pre-cluster resources
- variables set within packages with `setVariables` may be difficult to share across packages
- this would necessitate a separate binary with it's own CLI that would need its own release schedule and maintenance

> :warning: **NOTE**: The package manager could also be made to be OCI-only but would come with the same OCI pros/cons.

## Decision

> :warning: **NOTE**: This functionality was migrated to [uds-cli](https://github.com/defenseunicorns/uds-cli) - this ADR is kept here for historical purposes.

The current proposition (subject to change before acceptance) is **Zarf Bundles**, which a following PR will focus on and create a POC of.

In essense the `zarf-bundle.yaml` would look something like so:

```yaml
metadata:
  name: omnibus
  description: an example Zarf bundle
  version: 0.0.1
  architecture: amd64

packages:
  - repository: localhost:888/init
    ref: "###ZARF_BNDL_TMPL_INIT_VERSION###"
    optional-components:
      - git-server
  - repository: ghcr.io/defenseunicorns/packages/dubbd
    ref: 0.0.1 # OCI spec compliant reference
    # arch is not needed as it will use w/e arch is set in the bundle's metadata
    optional-components: # by default, all required components will be included
      - "*" # include all optional components
  - repository: docker.io/<namespace>/<name>
    ref: 0.0.1
    optional-components:
      - preflight
      - aws-* # include all optional components that start with "aws-"
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

This does add complexity to the Zarf codebase, as it is the addition of an entire suite of commands, JSON schema, schema docs, CLI docs, and a chunk of library code + tests.  It is a good litmus test of the current packager and OCI codebase to see how ready it is to be consumed as a library.

Additionally, this does add a new layer of complexity to the Zarf ecosystem, as meta-package maintainers must now also be aware of this bundling process, syntax and schema.  This is a necessary evil however, as the current pattern of using `zarf package deploy` to deploy multiple packages is not sustainable at the scale we are seeing.

There is also the hard requirement that packages bundled must be first published to a registry available to the person performing the bundle operation. This removes some ability to develop bundles on an air gapped environment, but the team believes that in such scenarios, the air gapped environment should be _receiving_ a bundle, rather than developing one internally.  If this assumption is incorrect however there are options for us to allow the creation of bundles from OCI directories on local systems if we need to or we could provide more provisions within Zarf to make it easier to connect to air gap registries to mirror bundles.

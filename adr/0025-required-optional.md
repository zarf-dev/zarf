# 25. Components can be required by default + introduction of feature flags

Date: 2024-04-29

## Status

Accepted

## Context

> Feature request: <https://github.com/defenseunicorns/zarf/issues/2059>

Currently, all Zarf components default to being optional due to the `required` key being _optional_ in the YAML. This leads to package authors needing to ensure that they annotate this key for each component, and since nothing in the current validations prompts them about this they may be confused about the "all things are optional" default state.

When Zarf was first created, we didn't really know how it would evolve and this key was introduced in those very early days. At this point it would be better to require all components by default--especially with the introduction of composability and the OCI skeleton work, there is plenty of flexibility in the API to compose bespoke packages assembled from other packages.

A few ways to handle this:

1. Simply force the `required` key to be a non-optional, so that package authors would be forced to specify it for each component, thereby removing any ambiguity--but also force one more key for every single component ever created 🫠

2. Deprecate `required` and introduce an optional `optional` key, which would default to _false_.

3. Do something more significant like combine various condition-based things such as `only`, `optional` (instead of `required`), or `default`.

4. Introduce `feature` flags to allow for certain schema behavior to be configurable to the user.

## Decision

Option 4. Introduce `.metadata.features` flags to `zarf.yaml` to allow for certain schema behavior to be configurable to the user.

The `features` key will be added to the `metadata` section of the package schema. This key will be an array of strings, where each string is the name of a beta feature that can be enabled. To enable a feature, the user will need to add the name of the feature to this array and edit the package schema accordingly.

> Such feature migrations can also be accomplished using `zarf dev migrate`, see the [Consequences](#consequences) section for more information.

e.g.

```diff
kind: ZarfInitConfig
metadata:
  name: init
  description: Used to establish a new Zarf cluster
+ features:
+   - default-required

components:
  - name: k3s
+   required: false
    import:
      path: packages/distros/k3s

  # This package moves the injector & registries binaries
  - name: zarf-injector
-   required: true
    import:
      path: packages/zarf-registry
```

## Consequences

The introduction of feature flags will allow Zarf to introduce new features and schema behavior without breaking existing packages and schemas, but also introduces more complexity. This will require more documentation and user education to ensure that users understand how to use these flags.

Beta feature flags will become the default behavior of Zarf upon the next major release, and will _not_ be configurable by the user at that time. This will allow for a more consistent experience across all Zarf packages.

There will be a flag added to the `zarf dev migrate` command `--enable-feature <feature-name>` to allow users to enable features on a per-package basis. This will allow users to test new features in a controlled environment before they are enabled by default.

> Tab autocompletion for the `--enable-feature` flag is enabled for the `zarf dev migrate` command.

e.g. (some output omitted for brevity)

```bash
$ zarf dev migrate --enable-feature default-required > migrated-zarf.yaml

 NOTE  Using config file ...

 NOTE  Saving log file to ...


     Migration        | Type         | Affected
     default-required | feature      | .

```

```diff
$ git diff --no-index zarf.yaml migrated-zarf.yaml

kind: ZarfInitConfig
 metadata:
   name: init
   description: Used to establish a new Zarf cluster
+  features:
+    - default-required
 components:
   - name: k3s
+    required: false
     import:
       path: packages/distros/k3s

   - name: zarf-injector
-    required: true
     import:
       path: packages/zarf-registry

   - name: zarf-seed-registry
-    required: true
     import:
       path: packages/zarf-registry

   - name: zarf-registry
-    required: true
     import:
       path: packages/zarf-registry

   - name: zarf-agent
-    required: true
     import:
       path: packages/zarf-agent

   - name: logging
+    required: false
     import:
       path: packages/logging-pgl

   - name: git-server
+    required: false
     import:
       path: packages/gitea
```

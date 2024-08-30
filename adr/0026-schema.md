# 25. Zarf Schema for v1

Date: 2024-06-07

## Status

Proposed

## Terms
v0 = any version of Zarf prior to v1

## Context

Zarf currently does not have explicit schema versions. Any schema changes are embedded into Zarf and can change with new versions. There are several examples of deprecated keys throughout Zarf's lifespan such as:

- `setVariable` deprecated in favor of `setVariables`
- `scripts` deprecated in favor of `actions`
- `group` deprecated in favor of `flavor`, however these work functionally different and cannot be automatically migrated
- `cosignKeyPath` deprecated in favor of specifying the key path at create time

Zarf has not disabled any deprecated keys thus far. On create the user is always warned when using a deprecated field, however the field still exists in the schema and functions properly. Some of the deprecated keys can be migrated automatically as the old item is a subset or directly related to it's replacement. For example, setVariable is automatically migrated to a single item list of setVariables. This migration occurs on the zarf.yaml fed into the package during package create, however the original field is not deleted from the packaged zarf.yaml because the Zarf binary used in the airgap or delivery environment is not assumed to have the new schema fields that the deprecated key was migrated to.

The release of v1 will provide an opportunity to delete deprecated features that Zarf has warned will be dropped in v1.

Creating a v1 schema will allow Zarf to establish a contract with it's user base that features will be supported long term. When a feature is deprecated in the v1 schema, it will remain usable in the schema for the lifetime of v1.

## Decision

Zarf will begin having proper schema versions. A top level key, `apiVersion`, will be introduced to allow users to specify the schema. At the release of v1 the only valid user input for `apiVersion` will be v1. Zarf will not allow users to build using the v0 schema. `zarf package create` will fail if the user has deprecated keys or if `apiVersion` is missing and the user will be instructed to run the new `zarf dev migrate-schema` command. `zarf dev migrate-schema` will automatically migrate deprecated fields in the users `zarf.yaml` where possible. It will also add the apiVersion key and set it to v1.

The existing go types which comprise the Zarf schema will be moved to types/alpha and will never change. An updated copy of these types without the deprecated fields will be created in a package types/v1 and any future schema changes will affect these objects. Internally, Zarf will introduce translation functions which will take the alpha schema and return the v1 schema. From that point on, all function signatures that have a struct that is included in the Zarf schema will change from `types.structName` to `v1.structName`.

All deprecated features will cause an error on create. Deprecated features with a direct migration path will still be deployed if the package was created with schema v1, as migrations will add the non deprecated fields. If a feature does not have a direct automatic migration path (cosignKeyPath & groups) the package will fail on deploy. This will happen until the alpha schema is entirely removed from Zarf, which will happen one year after v1 is released.

At create time Zarf will package both a `zarf.yaml` and a `zarfv1.yaml`. If a `zarfv1.yaml` exists Zarf will use that. If a `zarfv1.yaml` does not exist, then Zarf will know that the package was created prior to v1 and use the regular `zarf.yaml`. If the package is deployed with v0 it will read the `zarf.yaml` as normal even if the package has a `zarfv1.yaml`. This will make it simpler to drop deprecated items with migration paths from the v1 schema while remaining backwards compatible as those deprecated items will exist in the `zarf.yaml`

When Zarf introduces new keys that they are not ready to promise long term support for they will mark them as experimental in the schema. A key is assumed to be stable if it's not experimental or deprecated.

There are several other keys Zarf will deprecate which will have automated migrations to new fields
- `.metadata.aggregateChecksum` -> `.build.aggregateChecksum`
- `.components[x].required` -> `.components[x].optional`. Optional will default to false. This is a change in behavior as currently `required` defaults to true. However, automatic migration will work since we can set optional to true on any components which do not already have required.
- Metadata fields `image`, `source`, `documentation`, `url`, `authors`, `vendors` -> will become a map of `annotations`
- `noWait` -> `wait` which will default to true. This change will happen on both `.components.[x].manifests` and `components.[x].charts`
- `yolo` -> `airgap` which will default to true
- `.components.[x].actions.[default/onAny].maxRetries` -> `.components.[x].actions.[default/onAny].retries`
- `.components.[x].actions.[default/onAny].maxTotalSeconds` -> `.components.[x].actions.[default/onAny].timeout`, which must be in a [Go recognized duration string format](https://pkg.go.dev/time#ParseDuration)
- charts will change to avoid [current confusion with keys](https://github.com/defenseunicorns/zarf/issues/2245). Exactly one of the following fields will exist for each `components.[x].charts`.
```yaml
  helm:
    url: https://stefanprodan.github.io/podinfo
    RepoName: podinfo # replaces repoName since it's only applicable for helm repos

  git:
    url: https://stefanprodan.github.io/podinfo
    path: charts/podinfo

  oci:
    url: oci://ghcr.io/stefanprodan/charts/podinfo

  local:
   path: chart
```
- A new field called `.components.[x].healthChecks` will be introduced and will utilize [kstatus](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md) to wait for resources to be ready after deployment. Read [#2718](https://github.com/zarf-dev/zarf/issues/2718) for more detail.
- a new field `.components.[x].wait` will be introduced. It will default to true, and automatically run `healthchecks` for every object in the cluster.
- `.components.[x].extensions` will be removed in favor of `zarf dev generate bigbang`. See #2875

### BDD scenarios
The following are [behavior driven development](https://en.wikipedia.org/wiki/Behavior-driven_development) scenarios to provide context of what Zarf will do in specific situations given the above decisions.

#### v1 create with deprecated keys
- *Given* Zarf version is v1
- *and* the `zarf.yaml` has no apiVersion or deprecated keys
- *when* the user runs `zarf package create`
- *then* they will receive an error and be told to run `zarf dev migrate-schema` or how to migrate off cosign key paths or how to use flavors over groups depending on the error

#### v0 create -> v1 deploy
- *Given*: A package is created with Zarf v0
- *and* that package has deprecated keys that can be automatically migrated (required, scripts, & set variables)
- *when* the package is deployed with Zarf v1
- *then* the keys will be automatically migrated & the package will be deployed without error.

#### v0 create with removed feature -> v1 deploy
- *Given*: A package is created with Zarf v0
- *and* that package has deprecated keys that cannot be automatically migrated (groups, cosignKeyPath)
- *when* the package is deployed with Zarf v1
- *then* then deploy of that package will fail and the user will be instructed to update their package

#### v1 create -> v0 deploy
- *Given*: A package is created with Zarf v1
- *and* that package uses keys that don't exist in v0
- *when* the package is deployed with Zarf v0
- *then* Zarf v0 will deploy the package without issues. Additionally if the users is on a v0 version of Zarf after issue [2728](https://github.com/defenseunicorns/zarf/issues/2728) has been implemented, if there are fields unrecognized by the v0 schema, then the user will be warned they are deploying a package that has features that do not exist in the current version of Zarf.

## Consequences
- As long as the only deprecated features in a package have a migration path, and the package was built after the feature was deprecated so migrations were run, Zarf will be successful in both creating a package with v1 and deploying with v0, and creating a package with v0 and deploying with v1.
- Users of deprecated `group`, `cosignKeyPath`, and `action.Wait` keys outside of `onDeploy` might be frustrated if their packages, created v0, error out on Zarf v1, however this is preferable to unexpected behavior occurring in the cluster.
- Users may be frustrated that they have to run `zarf dev migrate-schema` to edit their `zarf.yaml` to remove the deprecated fields and add `apiVersion`.
- The Zarf codebase will contain two Zarf package objects, v1 and v0. Many fields on these objects will be unchanged across v0 and v1, however, v0 will not include new fields, and v1 will exclude deprecated fields. This approach is similar to the strategies used by [Kubernetes](https://github.com/kubernetes/api/tree/master/storage) and [flux](https://github.com/fluxcd/source-controller/tree/main/api)

Look at [0026-schema.yaml](0026-schema.yaml) to see an example v1 zarf.yaml with, somewhat, reasonable & nonempty values for every key.

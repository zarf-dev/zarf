# 13. Big Bang as a Noun

Date: 2023-01-18

## Status

Accepted

## Context

One primary application component that end users of Zarf are deploying is [Big Bang](https://repo1.dso.mil/big-bang/bigbang). The installation of Big Bang is complicated for several reasons:

- It requires Flux to be installed to deploy correctly due to the use of Flux CRDs.
- The [images](https://umbrella-bigbang-releases.s3-us-gov-west-1.amazonaws.com/umbrella/1.51.0/package-images.yaml) defined within Big Bang are normally a superset of the images needed for any individual deployment.
- All images that Big Bang might need takes 10s of gigabytes of storage to include in a Zarf package.
- The git repositories defined within Big Bang are normally a superset of the git repositories needed for any individual deployment.
- Injecting a `values.yaml` file into the [default deployment structure](https://repo1.dso.mil/big-bang/bigbang/-/blob/master/base/kustomization.yaml) is complicated and the discovery of which images are needed is a function of the values that are provided to the Big Bang chart

## Decision

Deployments of Big Bang can be managed with a new `bigbang` noun in the zarf.yaml that manages the complexity of the deployment. This capability will take the values provided to the big bang chart, template them during the package phase to identify which [Big Bang packages](https://repo1.dso.mil/big-bang/bigbang/-/blob/master/docs/packages.md) are being configured in the Zarf package. The code then includes only the git repositories and images needed for the configured packages, and does not include the git repositories and images for packages that would not be deployed.

The `bigbang` section will provide the following configurations for managing a big bang deployment:

- `version` - Identifies the particular version of Big Bang to deploy, which corresponds to git tags in the provided `repo`. See versions of Big Bang [here](https://repo1.dso.mil/big-bang/bigbang/-/releases).
- `repo` - Identifies the git repository Big Bang is hosted on. Defaults to https://repo1.dso.mil/big-bang/bigbang.git
- `valuesFiles` - list of local files that get passed to the Big Bang helm chart for deployment.
- `skipFlux` - boolean to determine if the flux installation for Big Bang should be skipped. Only set this to true if flux has been deployed in a different way already in the cluster.

## Implementation

The Big Bang component in the original zarf.yaml will update the component to install Flux (assuming the `skipFlux` flag is not set to true) and Big Bang. The flux deployment is just a remote Kustomization pointing at the [corresponding version of flux](https://repo1.dso.mil/big-bang/bigbang/-/tree/master/base/flux), with Big Bang resulting in the following objects being defined as manifests:

1. The `bigbang` `Namespace`
2. One `Secret` for each file provided in `valuesFiles`
3. `GitRepository` object pointing at the provided `repo`.
4. `HelmRelease` object deploying the chart from the `GitRepository` configured with the secrets as values files.

## Consequences

- By doing package time rendering and discovery of images for inclusion into the zarf package, the flexibility for deploy time configuration is limited since new parts of Big Bang can't be added arbitrarily, since the necessary artifacts to deploy those Big Bang packages won't be present in the zarf package
- Big Bang is ever changing and improving, and while it is available as open source, we do not control the change in how the deployment is handled as [Big Bang 2.0](https://repo1.dso.mil/groups/big-bang/-/epics/217) is progressing. This creates a burden on the Zarf team to ensure new changes in Big Bang do not break how Big Bang is deployed, and a burden to ensure as the way Big Bang gets deployed is changed, it does not break older versions of deploying Big Bang.

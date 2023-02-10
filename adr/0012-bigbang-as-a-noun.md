# 12. BigBang as a Noun

Date: 2023-01-18

## Status

Accepted

## Context

One primary application component that end users of Zarf are deploying is [Big Bang](https://repo1.dso.mil/big-bang/bigbang).  The installation of BigBang is complicated for several reasons:

- It requires Flux to be installed to deploy correctly due to the use of Flux CRDs. 
- The [images](https://umbrella-bigbang-releases.s3-us-gov-west-1.amazonaws.com/umbrella/1.51.0/package-images.yaml) defined within BigBang are normally a superset of the images needed for any individual deployment.
- All images that BigBang might need takes 10s of gigabytes of storage to include in a Zarf package.
- The git repositories defined within BigBang are normally a superset of the git repositories needed for any individual deployment.
- Injecting a `values.yaml` file into the [default deployment structure](https://repo1.dso.mil/big-bang/bigbang/-/blob/master/base/kustomization.yaml) is complicated and the discovery of which images are needed is a function of the values that are provided to the BigBang chart


## Decision

Deployments of BigBang can be managed with a new `bigbang` noun in the zarf.yaml that manages the complexity of the deployment.  This capability will take the values provided to the big bang chart, template them during the package phase to identify which [BigBang packages](https://repo1.dso.mil/big-bang/bigbang/-/blob/master/docs/packages.md) are being configured in the Zarf package.  The code then includes only the git repositories and images needed for the configured packages, and does not include the git repositories and images for packages that would not be deployed. 


 The `bigbang` section will provide the following configurations for managing a big bang deployment:

- `version` - Identifies the particular version of Bigbang to deploy, which corresponds to git tags in the provided `repo`.  See versions of BigBang [here](https://repo1.dso.mil/big-bang/bigbang/-/releases).  
- `repo` - Identifies the git repository BigBang is hosted on.  Defaults to https://repo1.dso.mil/big-bang/bigbang.git
- `valuesFrom` - list of local files that get passed to the BigBang helm chart for deployment. 
- `skipFlux` - boolean to determine if the flux installation for BigBang should be skipped.  Only set this to true if flux has been deployed in a different way already in the cluster.

## Implementation

The BigBang component in the origional zarf.yaml will be split into up to two different zarf components for packaging and deployment.  The first is the flux component, which will only be included if `skipFlux` is set to false (the default). The flux deployment is just a remote Kustomizatoin pointing at the [suggested installation of flux](https://repo1.dso.mil/big-bang/bigbang/-/tree/master/base/flux):   The BigBang component will be the following objects deployed as manifests:

1. The `bigbang` `Namespace`
2. One `Secret` for each file provided in `valuesFrom`
3. `GitRepository` object pointing at the provided `repo`.
4. `HelmRelease` object deploying the chart from the `GitRepository` configured with the secrets as values files.

## Consequences


- By doing package time rendering and discovery of images for inclusion into the zarf package, the flexibility for deploy time configuration is limited since new parts of BigBang can't be added arbitrarily, since the necessary artifacts to deploy those BigBang packages won't be present in the zarf package
- BigBang is every changing and improving, and while it is available as open source, we do not control the change in how the deployment is handled as [BigBang 2.0](https://repo1.dso.mil/groups/big-bang/-/epics/217) is progressing.  This creates a burden on the Zarf team to ensure new changes in Big Bang do not break how BigBang is deployed, and a burden to ensure as the way BigBang gets deployed is changed, it does not break older versions of deploying BigBang.


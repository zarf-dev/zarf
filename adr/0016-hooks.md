# 16. Zarf Hooks

Date: 2023-06-13

## Status

Pending

## Context

Zarf packages already have the concept of `actions` that can execute commands on the host machine's shell during certain package lifecycle events. As `actions` gain more adoption, the team has noticed they are being used to add functionality to Zarf in unexpected ways.  We want `actions` to be a tool that extends upon the functionality of Zarf and its packages, not a tool that works around missing or clunky functionality.


We want package creators to be able to create system agnostic packages by leveraging core Zarf functionality. The following is one such scenario:

- _IF_ ECR is chosen as the external registry during `zarf init` / cluster creation, _THEN_ Zarf will seamlessly leverage ECR without requiring advanced user effort.

Using ECR as a remote registry creates 2 problems that Zarf will need to solve:
 1. ECR authentication tokens expire after 12 hours and need to be refreshed. This means the cluster will need to constantly be refreshing its tokens and the user deploying packages will need to make sure they have a valid token.
 2. ECR Image Repositories do not support 'push-to-create'. This means we will need to explicitly create an image repository for every image that is being pushed within the Zarf package.

Packages that get deployed onto a cluster initialized with ECR as its remote registry will need to make sure it solves these 2 problems, either by having the package deployer do something prior to deploying the package or by having the package itself solve these problems with `actions` that are custom built for ECR clusters. Neither one of these current solutions are ideal. We don't want package deployers to have to do something outside of Zarf to get a Zarf package to work, and we don't want package creators to have to create packages that are specific to a certain image registry.


## Decision

The idea of `hooks` is to provide a way for packages to define functionality that runs on a package and component deployment lifecycle. Clusters that have a hook(s) will have a `zarf-hook-*` secret in the 'zarf' namespace. This secret will contain the hook's configuration and any other information that the hook needs to run. As part of the package deployment process, Zarf will check if the cluster has any hook secrets and run them if they exist. Given the scenario we considered above, having hooks will mean that instead need custom packages with ECR specific `actions`, we can have install hooks that will perform the ECR configuration for every package we deploy onto that cluster in the future.


## Implementation

Zarf HookConfig state information:

```go
type HookConfig struct {
	HookName     string                 `json:"hookName" jsonschema:"description=Name of the hook"`
	Internal     bool                   `json:"internal" jsonschema:"description=Internal hooks are run by Zarf itself, not by a plugin"`
	Lifecycle    HookLifecycle          `json:"lifecycle" jsonschema:"description=Lifecycle of the hook"`
	HookData     map[string]interface{} `json:"hookData" jsonschema:"description=Generic data map used for the hook. The data is obtained from a secret in the Zarf namespace"`
	OCIReference string                 `json:"ociReference" jsonschema:"description=Optional OCI reference to the hook image to run"`
}

```

Zarf hooks will have two forms of execution via `Internal` and `External` hooks. Internal hooks will be hooks that are built into the Zarf CLI and run internal code when executed. External hooks will reference a container image that will be downloaded and run. HookData will either be used by the internal hook or passed as a map to the external hook.

Hooks lifecycle options will be before/after a package deployment and before/after a component deployment.
 - NOTE: The order of hook execution is nearly random. If there are multiple hooks for a lifecycle there is no guarantee that they will be executed in a certain order.
 - NOTE: The `package` lifecycle might be changed to a `run-once` lifecycle. This would benefit packages that don't have kube context information when the deployment starts.

Hooks have to be 'installed' onto a cluster before they are used. When Zarf is deploying a package onto a cluster, it will look for any secrets with the `zarf-hook` label in the `zarf` namespace. If hooks are found, Zarf will run any 'package' level hooks before deploying a component and run any 'component' level hook for each component that is getting deployed.


## Consequences

- External hooks will likely not be implemented in the first pass of this feature. Handling external hooks will be an interesting challenge as we'll have to download the hook image and run execute it. Security will be something we consider heavily when implementing this feature.

- While hooks don't introduce raw schema changes to Zarf, it does add complexity where side affects are happening during package deployments that might not be obvious to the package deployer. This is especially the case if the person who deployed the hooks is different from the person who is deploying the subsequent packages.

- At the current moment, we don't have a way to version hooks. This is something we should consider so we can update hooks that have been deployed onto a cluster.

- At the current moment, there is no way to have a package opt out of running hooks. This means that someone who deploys a hook to a cluster effectively has a way to manipulate every other package deployment that will get deployed onto that cluster.

- Some situations will require hooks to be 'seeded' onto the cluster. For example, the ECR scenario we identified above would require hooks to exist before running `zarf init` on the EKS cluster.

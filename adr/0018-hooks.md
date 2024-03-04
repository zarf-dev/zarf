# 18. Zarf Hooks

Date: 2023-09-20

## Status

Accepted

## Context

The idea of `hooks` is to provide a way for cluster maintainers to register functionality that runs during the deployment lifecycle. Zarf packages already have the concept of `actions` that can execute commands on the host machine's shell during certain package lifecycle events. As `actions` gain more adoption, the team has noticed they are being used to add functionality to Zarf in unexpected ways. We want `actions` to be a tool that extends upon the functionality of Zarf and its packages, not a tool that works around missing or clunky functionality.

We want package creators to be able to create system agnostic packages by leveraging core Zarf functionality. The following is one such scenario:

- _IF_ ECR is chosen as the external registry during `zarf init` / cluster creation, _THEN_ Zarf will seamlessly leverage ECR without requiring advanced user effort.

Using ECR as a remote registry creates 2 problems that Zarf will need to solve:

 1. ECR authentication tokens expire after 12 hours and need to be refreshed. This means the cluster will need to constantly be refreshing its tokens and the user deploying packages will need to make sure they have a valid token.
 2. ECR Image Repositories do not support 'push-to-create'. This means we will need to explicitly create an image repository for every image that is being pushed within the Zarf package.

Packages that get deployed onto a cluster initialized with ECR as its remote registry will need to make sure it solves these 2 problems.

Currently there are 2 solutions:

1. The package deployer solves the problem pre-deployment (creating needed repos, secrets, etc...)
2. The package itself solves these problems with `actions` that are custom built for ECR clusters.

Neither one of these current solutions are ideal. We don't want to require overly complex external + prior actions for Zarf package deployments, and we don't want package creators to have to create and distribute packages that are specific to ECR.

Potential considerations:

### Internal Zarf Implementation

  Clusters that have hooks will have `zarf-hook-*` secret(s) in the 'zarf' namespace. This secret will contain the hook's configuration and any other required metadata. As part of the package deployment process, Zarf will check if the cluster has any hooks and run them if they exist. Given the scenario above, there is no longer a need for an ECR specific Zarf package to be created. An ECR hook would perform the proper configuration for any package deployed onto that cluster; thereby requiring no extra manual intervention from the package deployer.

  Zarf HookConfig state information struct:

  ```go
  type HookConfig struct {
    HookName     string                 `json:"hookName" jsonschema:"description=Name of the hook"`
    Internal     bool                   `json:"internal" jsonschema:"description=Internal hooks are run by Zarf itself, not by a plugin"`
    Lifecycle    HookLifecycle          `json:"lifecycle" jsonschema:"description=Lifecycle of the hook"`
    HookData     map[string]interface{} `json:"hookData" jsonschema:"description=Generic data map used for the hook. The data is obtained from a secret in the Zarf namespace"`
    OCIReference string                 `json:"ociReference" jsonschema:"description=Optional OCI reference to the hook image to run"`
  }
  ```

  Example Secret Data:

  ```yaml
  hookName: ecr-repository
  internal: true
  lifecycle: before-component
  hookData:
    registryURL: public.ecr.aws/abcdefg/zarf-ecr-registry
    region: us-east-1
    repositoryPrefix: ecr-zarf-registry
  ```

  For this solution, hooks have to be 'installed' onto a cluster before they are used. When Zarf is deploying a package onto a cluster, it will look for any secrets with the `zarf-hook` label in the `zarf` namespace.  If hooks are found, Zarf will run any 'package' level hooks before deploying a component and run any 'component' level hook for each component that is getting deployed. The hook lifecycle options will be:

  1. Before a package deployment
  2. After a package deployment
  3. Before a component deployment
  4. After a component deployment

  NOTE: The order of hook execution is nearly random. If there are multiple hooks for a lifecycle there is no guarantee that they will be executed in a certain order.
  NOTE: The `package` lifecycle might be changed to a `run-once` lifecycle. This would benefit packages that don't have kube context information when the deployment starts.

  Zarf hooks will have two forms of execution via `Internal` and `External` hooks:

  Internal Hooks:

  Internal hooks will be hooks that are built into the Zarf CLI and run internal code when executed. The logic for these hooks would be built into the Zarf CLI and would be updated with new releases of the CLI.

  External Hooks:

  There are a few approaches for external hooks:

  1. Have the hook metadata reference an OCI image that is downloaded and run.

     - The hook metadata can reference the shasum of the image to ensure the image is not tampered with.
     - We can pass metadata from the secret to the image.

  1. Have the hook metadata reference an image/endpoint that we call via a gRPC call.
     - This would require a lot of consideration to security since we will be executing code from an external source.

  1. Have the hook metadata contain a script or list of shell commands that can get run.
     - This would be the simplest solution but would require the most work from the hook creator. This also has the most potential security issues.

  Pros:

  - Implementing Hooks internally means we don't have to deal with any bootstrapping issues.
  - Internally managed hooks can leverage Zarf internal code.

  Cons:

  - Since 'Internal' hooks are built into the CLI, the only way to get updates for the hook is to  update the CLI.
  - External hooks will have a few security concerns that we will have to work through.
  - Implementing hooks internally adds more complexity to the Zarf CLI. This is especially true if we end up using WASM as the execution engine for hooks.

### Webhooks

  Webhooks, such as Pepr, can act as a K8s controller that enables Kubernetes mutations. We are (or will be) considering using Pepr to replace the `Zarf Agent`. Pepr is capable to accomplishing most of what Zarf wants to do with the concept of Hooks. Zarf hook configuration could be saved as secrets that Zarf will be able to use. As Zarf is deploying packages onto a cluster, it can check for secrets the represent hooks (as it would if hook execution is handled internally as stated above) and get information on how to run the webhook from the secret. This would likely mean that the secret that describes the hook would have a `URL` instead of an `OCIReference` as well as config information that it would pass through to the hook. With the webhook approach, lifecycle management is a lot more flexible as the webhook can operate on native kubernetes events such as a secret getting created / updated.

  Pros:

  - Pepr as a solution would be more flexible than the internal Zarf implementation of Hooks since the webhook could be anywhere.
  - Using Pepr would reduce the complexity of Zarf's codebase.
  - It will be easier to secure third party hooks when Pepr is the one running them.
  - Lifecycle management would be a lot easier with a webhook solution like Pepr.

  Cons:

  - Pepr is a new project that hasn't been stress tested in production yet (but neither has Hooks).
  - The Pepr image needs to be pushed to an image registry before it is deployed. This will require a new bootstrapping solution to solve the ECR problem we identified above.

## Decision

[Pepr](https://github.com/defenseunicorns/pepr) will be used to enable custom, or environment-specific, automation tasks to be integrated in the Zarf package deployment lifecycle. Pepr also allows the Zarf codebase to remain agnostic to any third-party APIs or dependencies that may be used.

A `--skip-webhooks` flag has been added to `zarf package deploy` to allow users to opt out of Zarf checking and waiting for any webhooks to complete during package deployments.

## Consequences

While hooks don't introduce raw schema changes to Zarf, it does add complexity where side affects are happening during package deployments that might not be obvious to the package deployer. This is especially the case if the person who deployed the hooks is different from the person who is deploying the subsequent packages.

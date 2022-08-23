# 5. Mutating Webhook

Date: 2022-05-18

## Status

Accepted

## Context

Currently Zarf leverages [Helm Post Rendering](https://helm.sh/docs/topics/advanced/#post-rendering) to mutate image paths and secrets for K8s to use the internal [Zarf Registry](../packages/zarf-registry/). This works well for simple K8s deployments where Zarf is performing the actual manifest apply but fails when using a secondary gitops tools suchs as [Flux](https://github.com/fluxcd/flux2), [ArgoCD](https://argo-cd.readthedocs.io/en/stable/), etc. At that point, Zarf is unable to provide mutation and it is dependent on the package author to do the mutations themselves using rudimentary templating. Further, this issue also exists when for CRDs that references the [git server](../packages/gitea/). A `zarf prepare` command was added previously to make this less painful, but it still requires additional burden on package authors to do something we are able to prescribe in code.

## Decision

A [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) is standard practice in K8s and there [are a lot of them](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#what-does-each-admission-controller-do). Using the normal Zarf component structure and deployment strategy we can leverage a mutating webhook to perform automatic imagePullSecret binding and image path updates as well as add additional as-needed mutations such as updating the [GitRepository](https://fluxcd.io/docs/components/source/gitrepositories/) CRD with the appropriate secret and custom URL for the git server if someone is using Flux.

## Consequences

While deploying the webhook will greatly reduce the package development burden, the nature of how helm manages resources still means we will have to be careful how we apply secrets that could collide with secrets deployed by helm with other tools. Additionally, to keep the webhook simple we are foregoing any side-effects in this iteration such as creating secrets on-demand in a namespace as it is created.  Adding side effects carries with it the need to roll those back on failure, handle additional RBAC in the cluster and integrate with the K8s API in the webhook. Therefore, some care will have to be taken for now with how registry and git secrets are generated in a namespace. For example, in the case of [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) these secrets can be created by those helm charts if we pass in the proper configuration.

Another benefit of this approach is another layer security for Zarf clusters.  The Zarf Agent will act as an intermediary not allowing images not in the Zarf Registry or git repos not stored in the internal git server.  

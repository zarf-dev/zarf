# 25. Mutating Webhook for Crossplane

Date: 2024-06-17

## Status

Proposed

## Context

> Feature Request <https://github.com/defenseunicorns/zarf/issues/2572>

[Crossplane](https://crossplane.io) a is tool to orchestrate control planes and allows composition of various resources to a complete IT stack.

### How is Crossplane being used with zarf
Crossplanes Capabilities are used to declaratively configure and provision cloud resources from the zarf cluster, and in some instances for the Zarf-enabled Cluster itself.

### What is the Problem
1. For [providers](#providers), [functions](#functions), and [configurations](#configurations) the Crossplane Pod reads the OCI images referenced to in their corresponding CRD.
<br>
Since currently Zarf-Agent does not patch these resources, Crossplane will attempt to pull from the unaltered source, usualy the internet.

1. Crossplane runs within the cluster and can therefore not use localhost:nodeport to access the OCI images

1. Crossplane creates the Deployments based on the contents of the image reference in their CRD. If it is rewritten it would create a Deployment with the rewritten image.
Due to problem #2 the Hostname would differ from the expected hostname by zarf. leading to invalid image referenes like `127.0.0.1:31999/upbound/provider-terraform:v0.16.0-zarf-255403924-zarf-928246402`

The proposed PR #2574 will enable rewriting the Crossplane resources in such a way, that they can be pulled using the cluster local service, instead of the nodeport.
It also adds and additional check to ensure a converted image reference with the in-cluster url will only replace the hostname. This ensures that the generated deployments will reference the local zarf registry without the need for the user to change any definitions.

### Why does Crossplane load OCI Images
OCI Images built for Crossplane Configurations, Functions, and Providers contain Yaml files which Crossplane can use to Configure the Cluster appropriately for the resource.
A Provider for example usually manages a bunch of CRDs, which are stored as part of the OCI image and deployed by Crossplane.

Further information on Crossplanes images can be found [here](https://github.com/crossplane/crossplane/blob/69fd693a2979e18dc980d9a9e10472d8f0394d09/contributing/specifications/xpkg.md).

## Decision
To adopt the change for the support of automatic rewrite of references to Crossplane packages.

## Consequences

### For Zarf Maintainers
The Zarf-Agent Mutating Admission Webhook manages the following Resources:
| Resource                 | Kind of replaced content              |
|:-------------------------|:--------------------------------------|
| Pod                      | Container Image, Registry Credentials |
| FluxCD Git Repository    | Git Repository, Git Credentials       |
| ArgoCD Application       | Git Repository                        |
| ArgoCD Repository        | Git Repository, Git Credentials       |
| Crossplane Provider      | Container Image, Registry Credentials |
| Crossplane Function      | Container Image, Registry Credentials |
| Crossplane Configuration | Container Image, Registry Credentials |

### For Zarf Users
see [side effects](#side-effects)

### For Platform Engineers
This change eliminates the need of developers of IT stacks with Crossplane to know about Zarf and provide separate handling if the Stacks will be deployed using Zarf. The use of Zarf as eventual deployment mechanism will be transparent to the development process of Crossplane stacks.

### Side Effects
PullSecrets will contain a in-cluster version additionally to the localhost reference.

If an Image is targeting the service where the registry is exposed it will be assumed to patched and only the hostname and port is patched.

## Additional Information

### Providers
Crossplane automates the use of various APIs through the concept of [providers](https://docs.crossplane.io/latest/concepts/providers/), which make use of desired state principles possible for any resource controlled through said API.

### Configurations
In Crossplane configurations can be used to package Composite Resource Definitions and Compositions and store them as OCI artefacts.
- While Providers use various APIs, [Composite Resource Definitions](https://docs.crossplane.io/v1.16/concepts/composite-resource-definitions/) can be used to create and Expose APIs in the Kubernetes Cluster.
- [Compositons](https://docs.crossplane.io/v1.16/concepts/compositions/) are a Template for specifying which [Managed Resources](https://docs.crossplane.io/v1.16/concepts/managed-resources/) should be created for resources using the corresponding Composite Resource Definition.

### Functions
Composition Functions increase flexibility and Logic with which the Managed Resources will be created.

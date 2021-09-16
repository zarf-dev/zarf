## Zarf Game Mode Example

This example demonstrates using Zarf with a postgres database in a kubernetes cluster.

### Choosing a Kubernetes Postgres Operator
#### Choices
1) CrunchyData Postgres Operator (https://github.com/CrunchyData/postgres-operator)
2) Zolando Postgres Operator (https://github.com/zalando/postgres-operator)
3) KubeDB (https://github.com/kubedb/operator)

#### Decision With Reasons
Zolano Postgres Operator
1) Pods in CruncyData are created as Deployments, instead of StatefulSets which can cause some odd behavior (see Credits #1)
2) KubeDB has an enterprise version, which is concerning since some features may be paywalled

### Steps to use:
1. Create a Zarf cluster as outlined in the main [README](../../README.md#2-create-the-zarf-cluster)
2. Follow [step 3](../../README.md#3-add-resources-to-the-zarf-cluster) using this config in this folder

### Credits:
-- https://blog.flant.com/comparing-kubernetes-operators-for-postgresql/
-- https://blog.flant.com/our-experience-with-postgres-operator-for-kubernetes-by-zalando/
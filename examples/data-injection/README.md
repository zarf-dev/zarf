## Zarf Appliance Mode Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no utility cluster and Zarf is simple a standard means of wrapping airgap concerns for K3s. This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy [Podinfo](https://github.com/stefanprodan/podinfo).

### Steps to use:
1. Create a Zarf cluster as outlined in the main [README](../../README.md#2-create-the-zarf-cluster)
2. Follow [step 3](../../README.md#3-add-resources-to-the-zarf-cluster) using this config in this folder

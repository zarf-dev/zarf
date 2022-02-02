## Zarf Appliance Mode Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no gitops service and Zarf is simply a standard means of wrapping airgap concerns for K3s. This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy [Podinfo](https://github.com/stefanprodan/podinfo).

### Steps to use:
1. Create a Zarf cluster as outlined in the main [README](../../README.md#2-create-the-zarf-cluster)
2. Run `zarf package create` in this directory to build this example package.
3. Run `zarf package deploy zarf-package-data-injection-demo.tar`

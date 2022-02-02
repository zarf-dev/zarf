## Zarf Appliance Mode Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no gitops service and Zarf is simply a standard means of wrapping airgap concerns for K3s. This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy [Podinfo](https://github.com/stefanprodan/podinfo).

### Steps to use:
1. Build everything you will need for this example
   1. `cd /path/to/zarf`
   2. `make build-cli init-package`
   3. `cd ./examples`
   4. `make package-example-data-injection`
   5. Either run `make vm-init` or roll your own Kubernetes cluster locally however you like.
2. Run `./zarf init` following the prompts as best fit for your environment 
   - If you did start up your own Kubernetes cluster say `yes` when prompted for k3s.
3. Run `./zarf package deploy zarf-package-data-injection-demo.tar`

# ConfigMap Test Tool with Zarf Injection

## Purpose

Bug reproduction tool that uses Zarf's injection mechanism to test ConfigMap creation and health checks in Kubernetes.

## What It Does

1. Extracts the Zarf init package (`zarf-init-amd64-v0.67.0-11-g0b411ed2.tar.zst`)
2. Calls `StartInjection` from Zarf's cluster package (creates injector pod and ConfigMaps)
3. Calls `StopInjection` (cleans up injector resources)
4. Creates a `local-registry-hosting` ConfigMap in the `kube-public` namespace
5. Runs a health check on the registry ConfigMap to verify it's ready

## Prerequisites

- The Zarf init package must be in the same directory: `zarf-init-amd64-v0.67.0-11-g0b411ed2.tar.zst`
- A running Kubernetes cluster (accessible via kubeconfig)
- The `zarf` namespace must exist in the cluster

## Build

```bash
cd hack/configmap-test
go mod tidy
go build -o configmap-test .
```

## Run

```bash
./configmap-test
```

No command-line flags are needed - all configuration is hardcoded.

## What Gets Created and Cleaned Up

### During Injection (temporary):
- Injector pod in `zarf` namespace
- Injector service (NodePort)
- Payload ConfigMaps in `zarf` namespace
- Rust binary ConfigMap

### After StopInjection (cleaned up):
All injector resources are removed.

### Remaining in cluster:
- `local-registry-hosting` ConfigMap in `kube-public` namespace

## Verification

Check the registry ConfigMap:

```bash
kubectl get cm -n kube-public local-registry-hosting -o yaml
```

## Cleanup

To remove the registry ConfigMap:

```bash
kubectl delete cm -n kube-public local-registry-hosting
```

## How It Works

This tool follows the same injection pattern as Zarf's init process:
1. Loads the init package from the tar.zst file
2. Uses `cluster.NewWithWait()` to connect to the cluster
3. Calls `c.StartInjection()` which creates all necessary injector resources
4. Calls `c.StopInjection()` to clean up injector resources
5. Creates a test ConfigMap and validates it with health checks

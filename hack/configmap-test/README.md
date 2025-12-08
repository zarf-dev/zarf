# ConfigMap Test Tool

## Purpose

Simple bug reproduction tool for testing ConfigMap creation and health checks in Kubernetes.

## What It Does

1. Creates 30 ConfigMaps with 1MB of random data each in the `configmap-test` namespace
2. Creates a single `local-registry-hosting` ConfigMap in the `kube-public` namespace
3. Runs a health check on the registry ConfigMap to verify it's ready
4. Leaves all ConfigMaps in the cluster (no cleanup)

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

## What Gets Created

### In `configmap-test` namespace:
- `test-configmap-00` through `test-configmap-29`
- Each contains 1MB of random binary data

### In `kube-public` namespace:
- `local-registry-hosting` with registry configuration

## Verification

Check the created ConfigMaps:

```bash
# List all test ConfigMaps
kubectl get cm -n configmap-test

# View the registry ConfigMap
kubectl get cm -n kube-public local-registry-hosting -o yaml
```

## Cleanup

To remove the test ConfigMaps:

```bash
# Delete test namespace (removes all 30 ConfigMaps)
kubectl delete namespace configmap-test

# Delete registry ConfigMap
kubectl delete cm -n kube-public local-registry-hosting
```

# External Infrastructure Tests

This package contains integration tests for Zarf operating against pre-existing external infrastructure — i.e. a registry and git server that exist outside (and before) the target cluster. This mirrors cloud or enterprise environments where infrastructure is provisioned independently of the cluster.

There are two test suites:

- **`ext_in_cluster_test.go`** — infrastructure running inside the same cluster as Zarf
- **`ext_out_cluster_test.go`** — infrastructure running outside the cluster

## Out-of-cluster infrastructure

The out-of-cluster suite uses `docker compose` to manage:

| Service | Image | Purpose |
|---|---|---|
| `gitea.localhost` | `gitea/gitea:1.18.1` | External git server |
| `registry.localhost` | `registry:3` | External OCI registry with htpasswd auth |
| `registry.init` | `httpd:2` | One-shot init container that generates the htpasswd file |

The compose file also owns the `k3d-k3s-external-test` Docker network (`172.31.0.0/16`), which both the registry and the k3d cluster join.

Credentials used throughout:

| Variable | Value |
|---|---|
| Registry push user | `push-user` |
| Git user | `git-user` |
| Password (all) | `superSecurePassword` |

## Running locally

These steps replicate what the test suite does automatically, useful for ad-hoc debugging. An important note for local development is that the `--registry-url` address that is used during init must be resolvable by both the development host and the cluster. For apple silicon with a container runtime that uses a linux vm - we need to ensure this resolves.

As such we use `*.localhost` as container names as they will be resolvable in a docker network while also supporting the MacOS pattern of resolving them to localhost. So `registry.localhost:5001` will be resolvable by both the host and the container runtime such as K3d.

K3d is used as it exposes the ability to set a network natively. Port 5001 is used as 5000 is reserved on MacOS.

### Prerequisites

- [`k3d`](https://k3d.io)
- `docker` with Compose v2 (`docker compose`)
- A built Zarf binary at `build/zarf`

### 1. Start external infrastructure

From this directory (`src/test/external`):

```bash
docker compose up -d
```

This creates the Docker network, starts Gitea, and starts the registry. The registry will not start until `registry.init` has finished writing the htpasswd file.

Verify the registry is ready:

```bash
wget -q -O- \
  http://push-user:superSecurePassword@localhost:5001/v2/_catalog
# expected: {"repositories":[]}
```

### 2. Create the k3d cluster

The cluster must join the compose network and be configured to resolve the registry mirror:

```bash
REGISTRY_CONFIG=$(mktemp /tmp/registries-XXXXXX.yaml)

cat > "$REGISTRY_CONFIG" <<EOF
mirrors:
  "registry.localhost:5001":
    endpoint:
      - http://registry.localhost:5001
EOF

k3d cluster create zarf-external-test \
  --network k3d-k3s-external-test \
  --host-alias 172.31.0.99:gitea.localhost \
  --registry-config "$REGISTRY_CONFIG"

rm -f "$REGISTRY_CONFIG"
```

### 3. Run `zarf init` against the external infrastructure

```bash
zarf init \
  --registry-url=registry.localhost:5001/test \
  --registry-push-username=push-user \
  --registry-push-password=superSecurePassword \
  --git-url=http://gitea.localhost:3000 \
  --git-push-username=git-user \
  --git-push-password=superSecurePassword \
  --confirm
```

### Cleanup

```bash
k3d cluster delete zarf-external-test
docker compose down
```

`docker compose down` removes the containers, the `registry-auth` volume, and the `k3d-k3s-external-test` network.

# Postgres Operator

This example demonstrates deploying a performant and highly available PostgreSQL database to a Zarf airgap cluster. It uses Zalando's [postgres-operator](https://github.com/zalando/postgres-operator) and provides the Postgres Operator UI and a deployment of PGAdmin for demo purposes.

## Tool Choice

After looking at several alternatives, Zalando's postgres operator felt like the best choice. Other tools that were close runners-up were the postgres-operator by [CrunchyData](https://github.com/CrunchyData/postgres-operator) and [KubeDB](https://github.com/kubedb/operator).

## Prerequisites

1. Install [Docker](https://docs.docker.com/get-docker/). Other container engines will likely work as well but aren't actively tested by the Zarf team.

1. Install [KinD](https://github.com/kubernetes-sigs/kind). Other Kubernetes distros will work as well, but we'll be using KinD for this example since it is easy and tested frequently and thoroughly.

1. Clone the Zarf project &mdash; for the example configuration files.

1. Download a Zarf release &mdash; you need a binary _**and**_ an init package, [here](../../docs/workstation.md#just-gimmie-zarf). <!-- TODO: non-existent -->

1. Log `zarf` into Iron Bank if you haven't already &mdash; instructions [here](../../docs/ironbank.md#2-configure-zarf-the-use-em). Optional for this specific example since the container comes from GitHub rather than Iron Bank but a good practice and needed for most of the other examples.

1. (Optional) Put `zarf` on your path &mdash; _technically_ optional but makes running commands simpler. Make sure you are picking the right binary that matches your system architecture. `zarf` for x86 Linux, `zarf-mac-intel` for x86 MacOS, `zarf-mac-apple` for M1 MacOS.

1. Create a Zarf cluster as described in the [Doom example docs](../game/README.md)

## Instructions

### Deploy the package

```sh
# Open the directory
cd examples/postgres-operator

# Build the package
zarf package create

# Deploy the package (Press TAB for the listing of available packages)
zarf package deploy
```

Wait a couple of minutes. You'll know it is done when Zarf exits and you get the 3 connect commands.

### Create the backups bucket in MinIO (TODO: Figure out how to create the bucket automatically)

1. Run `zarf connect minio` to navigate to the web console.
1. Log in - Username: `minio` - Password: `minio123`
1. Buckets -> Create Bucket
   - Bucket Name: `postgres-operator-backups`

### Open the UI

The Postgres Operator UI will be available by running `./zarf connect postgres-operator-ui` and pgadmin will be available by running `./zarf connect pgadmin`

> ⚠️ **NOTE:** *If you want to run other commands after/during the browsing of the postgres tools, you can add a `&` character at the end of the connect command to run the command in the background ie) `./zarf connect pgadmin &`.*

### Set up a server in PGAdmin:
  - General // Name: `acid-zarf-test`
  - General // Server group: `Servers`
  - Connection // Host: (the URL in the table below)
  - Connection // Port: `5432`
  - Connection // Maintenance database: `postgres`
  - Connection // Username: `zarf`
  - Connection // Password: (run the command in the table below)
  - SSL // SSL mode: `Require`

### Clean Up

```sh
kind delete cluster
```

## Logins

| Service                   | URL                                                                                        | Username             | Password                                                                                                                                                   |
| ------------------------- | ------------------------------------------------------------------------------------------ | -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Postgres Operator UI      | `zarf connect postgres-operator-ui` | N/A                  | N/A                                                                                                                                                        |
| PGAdmin                   | `zarf connect pgadmin`                           | `zarf@example.local` | Run: `zarf tools get-admin-password`                                                                                                                       |
| Example Postgres Database | `acid-zarf-test.postgres-operator.svc.cluster.local`                                       | `zarf`               | Run: `echo $(kubectl get secret zarf.acid-zarf-test.credentials.postgresql.acid.zalan.do -n postgres-operator --template={{.data.password}} \| base64 -d)` |
| Minio Console             | `zarf connect minio`               | `minio`              | `minio123`                                                                                                                                                 |

## References
- https://blog.flant.com/comparing-kubernetes-operators-for-postgresql/
- https://blog.flant.com/our-experience-with-postgres-operator-for-kubernetes-by-zalando/

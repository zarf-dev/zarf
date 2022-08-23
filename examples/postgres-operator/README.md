# Postgres Operator

This example demonstrates deploying a performant and highly available PostgreSQL database to a Zarf airgap cluster. It uses Zalando's [postgres-operator](https://github.com/zalando/postgres-operator) and provides the Postgres Operator UI and a deployment of PGAdmin for demo purposes.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

:::note

This example uses Zalando's postgres operator as after looking at several alternatives, this felt like the best choice. Other tools that were close runners-up were the postgres-operator by [CrunchyData](https://github.com/CrunchyData/postgres-operator) and [KubeDB](https://github.com/kubedb/operator).

:::

&nbsp;

## Prerequisites

1. Install [Docker](https://docs.docker.com/get-docker/). Other container engines will likely work as well but aren't actively tested by the Zarf team.

1. Install [KinD](https://github.com/kubernetes-sigs/kind). Other Kubernetes distros will work as well, but we'll be using KinD for this example since it is easy and tested frequently and thoroughly.

1. Clone the Zarf project &mdash; for the example configuration files.

1. Build the package using `zarf package create examples/postgres-operator`

1. Create a Zarf cluster as described in the [Initializing a Cluster Walkthrough](../../docs/13-walkthroughs/1-initializing-a-k8s-cluster.md/)

&nbsp;

## Instructions

&nbsp;

### Deploy the package

Run the following commands to deploy the created package to the cluster

```sh
# Open the directory
cd examples/postgres-operator

# Build the package
zarf package create

# Deploy the package (Press TAB for the listing of available packages)
zarf package deploy
```

Wait a couple of minutes. You'll know it is done when Zarf exits and you get the 3 connect commands.


&nbsp;

### Create the backups bucket in MinIO (TODO: Figure out how to create the bucket automatically)

1. Run `zarf connect minio` to navigate to the web console.
1. Log in - Username: `minio` - Password: `minio123`
1. Buckets -> Create Bucket
   - Bucket Name: `postgres-operator-backups`

&nbsp;

### Open the UI

The Postgres Operator UI will be available by running `./zarf connect postgres-operator-ui` and pgadmin will be available by running `./zarf connect pgadmin`

:::note

If you want to run other commands after/during the browsing of the postgres tools, you can add a `&` character at the end of the connect command to run the command in the background ie) `./zarf connect pgadmin &`.

:::

&nbsp;

### Set up a server in PGAdmin:
  - General // Name: `acid-zarf-test`
  - General // Server group: `Servers`
  - Connection // Host: (the URL in the table below)
  - Connection // Port: `5432`
  - Connection // Maintenance database: `postgres`
  - Connection // Username: `zarf`
  - Connection // Password: (run the command in the table below)
  - SSL // SSL mode: `Require`

&nbsp;

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

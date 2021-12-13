# Zarf Postgres Operator Example

This example demonstrates deploying a performant and highly available PostgreSQL database to a Zarf airgap cluster. It uses Zalando's [postgres-operator](https://github.com/zalando/postgres-operator) and provides the Postgres Operator UI and a deployment of PGAdmin for demo purposes.

:warning: NOTE: It looks like this example doesn't currently quite work. The operators come up but it doesn't deploy a postgres database like it used to. We are working on a fix.

## Tool Choice

After looking at several alternatives, Zalando's postgres operator felt like the best choice. Other tools that were close runners-up were the postgres-operator by [CrunchyData](https://github.com/CrunchyData/postgres-operator) and [KubeDB](https://github.com/kubedb/operator).

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`
1. Install `sha256sum` (on Mac it's `brew install coreutils`)

## Instructions

1. `cd examples/postgres-operator`
1. Run one of these two commands:
  - `make all` - Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
  - `make all-dev` - Build Zarf locally, build the deploy package, and start a VM with Vagrant
1. Run: `./zarf init --confirm --components management --host 127.0.0.1` - Initialize Zarf, telling it to install just the management component, and tells Zarf to use `127.0.0.1` as the hostname. If you want to use interactive mode instead just run `./zarf init`.
1. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
1. Run: `./zarf package deploy zarf-package-postgres-operator-demo.tar.zst --confirm` - Deploy the package. If you want interactive mode instead just run `./zarf package deploy`, it will give you a picker to choose the package.
1. Wait a couple of minutes. Run `k9s` to watch progress
1. The Postgres Operator UI will be available at [https://postgres-operator-ui.localhost:8443](https://postgres-operator-ui.localhost:8443) and PGAdmin will be available at [https://pgadmin.localhost:8443](https://pgadmin.localhost:8443).
1. Set up a server in PGAdmin:
  - General // Name: `acid-zarf-test`
  - General // Server group: `Servers`
  - Connection // Host: (the URL in the table below)
  - Connection // Port: `5432`
  - Connection // Maintenance database: `postgres`
  - Connection // Username: `zarf`
  - Connection // Password: (run the command in the table below)
  - SSL // SSL mode: `Require`
1. Create the backups bucket in MinIO (TODO: Figure out how to create the bucket automatically)
  1. Navigate to [https://minio-console.localhost:8443](https://minio-console.localhost:8443)
  1. Log in - Username: `minio` - Password: `minio123`
  1. Buckets -> Create Bucket
    - Bucket Name: `postgres-operator-backups`
1. When you're done, run `exit` to leave the VM then `make vm-destroy` to bring everything down



## Logins

| Service                   | URL                                                                                        | Username             | Password                                                                                                                                                   |
| ------------------------- | ------------------------------------------------------------------------------------------ | -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Postgres Operator UI      | [https://postgres-operator-ui.localhost:8443](https://postgres-operator-ui.localhost:8443) | N/A                  | N/A                                                                                                                                                        |
| PGAdmin                   | [https://pgadmin.localhost:8443](https://pgadmin.localhost:8443)                           | `zarf@example.local` | Run: `zarf tools get-admin-password`                                                                                                                       |
| Example Postgres Database | `acid-zarf-test.postgres-operator.svc.cluster.local`                                       | `zarf`               | Run: `echo $(kubectl get secret zarf.acid-zarf-test.credentials.postgresql.acid.zalan.do -n postgres-operator --template={{.data.password}} \| base64 -d)` |
| Minio Console             | [https://minio-console.localhost:8443](https://minio-console.localhost:8443)               | `minio`              | `minio123`                                                                                                                                                 |

## References
- https://blog.flant.com/comparing-kubernetes-operators-for-postgresql/
- https://blog.flant.com/our-experience-with-postgres-operator-for-kubernetes-by-zalando/

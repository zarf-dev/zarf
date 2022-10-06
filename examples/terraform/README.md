# Terraform
This example demonstrates how to use Zarf to execute Terraform code to create an S3 bucket.

### Assumptions/Prereqs
- The binaries in the Zarf package are for an M1 Mac only and will need to be changed for other architectures
- The S3 bucket name will likely need to be changed as S3 bucket names must be globally unique
- Your machine has a connection to an AWS instance and is authenticated with an AWS account

### Steps

No K8s cluster is necessary, just build with the package with:

`zarf package create`

And execute with:

`zarf package deploy <package_name>`

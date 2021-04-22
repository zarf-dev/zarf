#!/bin/bash
set -e

TERRAFORM_VERSION="0.14.10"
BIG_BANG_RELEASE="1.5.0"

# Ensure the directory is clean
rm -fr ".airgap"
mkdir ".airgap"
pushd .airgap
mkdir bin images infra rpms 

# TODO: unzip this file
# Download terraform
curl -L "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" -o "bin/terraform"

# Get the Big Bang assets
curl -L "https://umbrella-bigbang-releases.s3-us-gov-west-1.amazonaws.com/umbrella/${BIG_BANG_RELEASE}/{images.txt,images.tar.gz,repositories.tar.gz}" -o "#1"
mv images.tar.gz images

# Ensure all binaries are executable
chmod +x bin/*

# Capture terraform resources
cp ../infra/* infra/
docker run -v $PWD:$PWD -w $PWD/infra -it hashicorp/terraform:${TERRAFORM_VERSION} init

# Ugh pull / save images for now for k3s
while read src; do
  docker pull "${src}"
  docker save --output "images/${src//[^[:alnum:]]/_}.tar" "${src}"
done < images.txt

popd

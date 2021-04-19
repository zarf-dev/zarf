#!/bin/bash
set -e

K3S_VERSION="v1.18.17+k3s1"
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

# Download the k3s assets
curl -fL "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/{k3s,k3s-airgap-images-amd64.tar,k3s-images.txt,sha256sum-amd64.txt}" -o "images/#1" && \
      ( pushd images || exit ; sha256sum -c sha256sum-amd64.txt )

# Get the K3s installer/binary in the bin folder
mv images/k3s bin/k3s 
curl -L "https://get.k3s.io/" -o "bin/k3s-install"

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

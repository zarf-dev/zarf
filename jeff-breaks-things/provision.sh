#!/bin/bash
set -e

# Disable firewalld for k3s
# https://rancher.com/docs/k3s/latest/en/advanced/#additional-preparation-for-red-hat-centos-enterprise-linux
if [ -f /etc/redhat-release ]; then
    systemctl disable firewalld --now
    yum localinstall -y --disablerepo=* --exclude container-selinux-1* /opt/shift/rpms/*.rpm
fi

# This is an airgap installation
export INSTALL_K3S_SKIP_DOWNLOAD=true

# Write the kubeconfig file
export K3S_KUBECONFIG_MODE="644"

# Copy the binaries to the path and prevent K3s from choking when trying to write to it later
cp /opt/shift/bin/* /usr/local/bin

# Add symlink for k3s images
mkdir -p /var/lib/rancher/k3s/agent/
ln -s /opt/shift/images /var/lib/rancher/k3s/agent/images

# install k3s
/usr/local/bin/k3s-install --disable=metrics-server --disable=traefik

# Deploy Flux
pushd /opt/shift/infra
terraform apply -auto-approve
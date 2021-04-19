#!/bin/bash
set -e

# docker run -v $PWD:/opt/shift \
#     -e RHEL_USER='' \
#     -e RHEL_PASS='' \
#     registry.access.redhat.com/ubi7/ubi \
#    sh -c /opt/shift/airgap-rhel.sh

echo "Setting up the RHEL Repos, this will take a minute"

subscription-manager register --auto-attach --username=$RHEL_USER --password=$RHEL_PASS
subscription-manager repos --enable=rhel-7-server-extras-rpms

yumdownloader --resolve --destdir=/opt/shift/.airgap/rpms/ container-selinux

# Download the K3S SELinux RPM 
curl -L "https://github.com/k3s-io/k3s-selinux/releases/download/v0.3.stable.0/k3s-selinux-0.3-0.el7.noarch.rpm" -o "/opt/shift/.airgap/rpms/k3s-selinux.rpm"

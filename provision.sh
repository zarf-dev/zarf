#!/bin/bash
set -e

setupDependencies() {
  configureVIP

  # https://rancher.com/docs/k3s/latest/en/advanced/#additional-preparation-for-red-hat-centos-enterprise-linux
  if [ -f /etc/redhat-release ]; then
    # info "Setting up dependencies for a RHEL-based distro"
    systemctl disable firewalld --now
    yum localinstall -y --disablerepo=* --exclude container-selinux-1* /opt/shift/rpms/*.rpm
  fi

}

configureVIP
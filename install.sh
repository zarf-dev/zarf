#!/bin/bash

set -e

K3S_DIR="/var/lib/rancher/k3s"

# info logs the given argument at info log level.
info() {
    tput setaf 6
    echo "[INFO] " "$@"
    tput sgr0
}

# warn logs the given argument at warn log level.
warn() {
    tput setaf 3
    echo "[WARN] " "$@" >&2
    tput sgr0    
}

# fatal logs the given argument at fatal log level.
fatal() {
    tput setaf 1
    echo "[ERROR] " "$@" >&2
    tput sgr0    
    exit 1
}

timestamp() {
  date "+%Y-%m-%d %H:%M:%S"
}

configureVIP() {
  info "Discovering ethernet network interface name"
  vipi=$(ip -o addr show scope global | awk '/^[0-9]:/{print $2}' | cut -f1 -d '/')

  info "Allocating vip on $vipi"
  vipa=$(ip addr show |grep -w inet |grep -v 127.0.0.1|awk '{ print $4}')

  info "Telling kube-vip about what we found"
  find . -type f -name "kube-vip.yaml" -exec sed -i -e 's|$VIP_INTERFACE|'$vipi'|g' -e 's|$VIP_ADDRESS|'$vipa'|g' {} \;
}

setupDependencies() {
  configureVIP

  # https://rancher.com/docs/k3s/latest/en/advanced/#additional-preparation-for-red-hat-centos-enterprise-linux
  if [ -f /etc/redhat-release ]; then
    info "Setting up dependencies for a RHEL-based distro"
    systemctl disable firewalld --now
    yum localinstall -y --disablerepo=* --exclude container-selinux-1* /opt/shift/rpms/*.rpm
  fi

  info "Moving k3s components..."
  mv rancher/ /var/lib/
  chmod -R 0700 /var/lib/rancher

  info "Moving k3s executable..."
  mv -f k3s/{k3s,init-k3s.sh} /usr/local/bin
}

installK3s() {
  info "Install K3s"
  K3S_KUBECONFIG_MODE="644" \
  INSTALL_K3S_SKIP_DOWNLOAD=true \
      /usr/local/bin/init-k3s.sh --disable=metrics-server --disable=traefik

  info "Setup kubectl autocompletion"
  /usr/local/bin/k3s kubectl completion bash >/etc/bash_completion.d/kubectl
}

setupDependencies
installK3s
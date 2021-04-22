#!/bin/bash
set -e

configureVIP() {
  # info "Discovering ethernet network interface name"
  vipi=$(ip -o addr show scope global | awk '/^[0-9]:/{print $2}' | cut -f1 -d '/')

  # info "Allocating vip on $vipi"
  vipa=$(ip addr show |grep -w inet |grep -v 127.0.0.1|awk '{ print $4}')

  # info "Telling kube-vip about what we found"
  find . -type f -name "kube-vip.yaml" -exec sed -i -e 's|$VIP_INTERFACE|'$vipi'|g' -e 's|$VIP_ADDRESS|'$vipa'|g' {} \;
}

setupDependencies() {
  configureVIP

  # https://rancher.com/docs/k3s/latest/en/advanced/#additional-preparation-for-red-hat-centos-enterprise-linux
  if [ -f /etc/redhat-release ]; then
    # info "Setting up dependencies for a RHEL-based distro"
    systemctl disable firewalld --now
    yum localinstall -y --disablerepo=* --exclude container-selinux-1* /opt/shift/rpms/*.rpm
  fi

}

installK3s() {
  # info "Install K3s"
  K3S_KUBECONFIG_MODE="644" \
  INSTALL_K3S_SKIP_DOWNLOAD=true \
      /usr/local/bin/init-k3s.sh --disable=metrics-server --disable=traefik

  # info "Setup kubectl autocompletion"
  /usr/local/bin/k3s kubectl completion bash >/etc/bash_completion.d/kubectl
}

setupDependencies
installK3s
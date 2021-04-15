#!/bin/bash

set -e

K3S_DIR="/var/lib/rancher/k3s"

# info logs the given argument at info log level.
info() {
    echo "[INFO] " "$@"
}

# warn logs the given argument at warn log level.
warn() {
    echo "[WARN] " "$@" >&2
}

# fatal logs the given argument at fatal log level.
fatal() {
    echo "[ERROR] " "$@" >&2
    exit 1
}

timestamp() {
  date "+%Y-%m-%d %H:%M:%S"
}

vip() {
  info "Discovering ethernet network interface name"
  vipi=$(ip -o addr show scope global | awk '/^[0-9]:/{print $2}' | cut -f1 -d '/')

  info "Allocating vip on $vipi"
  vipa=$(ip addr show |grep -w inet |grep -v 127.0.0.1|awk '{ print $4}')

  info "Telling kube-vip about what we found"
  find . -type f -name "kube-vip.yaml" -exec sed -i -e 's|$VIP_INTERFACE|'$vipi'|g' -e 's|$VIP_ADDRESS|'$vipa'|g' {} \;
}

setup() {
  vip

  info "Ensuring k3s directory is empty..."
  rm -rf /var/lib/rancher

  info "Moving k3s components..."
  mv rancher/ /var/lib/
  chmod -R 0755 /var/lib/rancher
  chmod -R 0700 /var/lib/rancher/k3s/server

  # Create default k3s config if doesn't already exist
  if [ ! -f "/etc/rancher/k3s/config.yaml" ]; then
    mkdir -p /etc/rancher/k3s
    chmod -R 0755 /etc/rancher
    chmod 0644 k3s-config.yaml
    mv k3s-config.yaml /etc/rancher/k3s/config.yaml
  fi

  # TODO: k3s supports selinux but this utility packaging script does not (yet)
  if getenforce 2>/dev/null | grep -q "Enforcing"; then
    info "Identified selinux enforcing, ensure k3s-selinux policies are pre-installed if you are offline, otherwise the policies will be installed for you from the internet."

    # SUUUUUUUPER basic check to see if k3s selinux policies are installed or not
    if ! semanage fcontext -l | grep -i k3s > /dev/null; then
      warn "No k3s selinux policies found and selinux is set to Enforcing.  To continue, either install the appropriate k3s selinux policies, or set selinux to Permissive"

      warn "No k3s selinux policies found and selinux is set to Enforcing, attempting to download k3s-selinux policies from the internet."
      warn "This download attempt WILL fail if in an airgapped environment."
      warn "If in a disconnected environment, install the airgapped k3s-selinux policy rpms first before running YAM."

      case ${maj_ver} in
      7)
        cat > /etc/yum.repos.d/rancher-k3s-common.repo <<EOF
[rancher-k3s-common-stable]
name=Rancher K3s Common (stable)
baseurl=https://rpm.rancher.io/k3s/stable/common/centos/7/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
        # yum install -y rpms/centos7/*.rpm
        yum install -y k3s-selinux
        ;;
      8)
        # yum install -y rpms/centos8/*.rpm
        cat > /etc/yum.repos.d/rancher-k3s-common.repo <<EOF
[rancher-k3s-common-stable]
name=Rancher K3s Common (stable)
baseurl=https://rpm.rancher.io/k3s/stable/common/centos/8/noarch
enabled=1
gpgcheck=1
gpgkey=https://rpm.rancher.io/public.key
EOF
        yum install -y k3s-selinux
        ;;
      esac
    else
      info "k3s-selinux policies found!"
    fi
  fi

  info "Moving k3s executable..."
  chmod 755 k3s/{k3s,init-k3s.sh}
  chown root:root k3s/k3s
  mv -f k3s/{k3s,init-k3s.sh} /usr/local/bin
}

start() {
  # Start k3s
  INSTALL_K3S_SKIP_DOWNLOAD=true \
  INSTALL_K3S_SELINUX_WARN=true \
  INSTALL_K3S_SKIP_SELINUX_RPM=true \
      /usr/local/bin/init-k3s.sh

  # Setup kubectl autocompletion
  /usr/local/bin/k3s kubectl completion bash >/etc/bash_completion.d/kubectl
}

{
  setup
  start
}

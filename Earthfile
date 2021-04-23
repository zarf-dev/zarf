# Earthfile

centos7-k3s-selinux-rpms:
  FROM centos:7.9.2009
  WORKDIR /deps

  RUN yum install yum-utils -y
  WORKDIR /rpms

  RUN echo $'\n\
  [rancher-k3s-common-stable]\n\
  name=Rancher K3s Common (stable)\n\
  baseurl=https://rpm.rancher.io/k3s/stable/common/centos/7/noarch\n\
  enabled=1\n\
  gpgcheck=1\n\
  gpgkey=https://rpm.rancher.io/public.key'\
  >> /etc/yum.repos.d/rancher-k3s-common.repo

  RUN yumdownloader --assumeyes --resolve --destdir=/rpms k3s-selinux

  WORKDIR /
  RUN tar -czvf rpms.tar.gz /rpms

  SAVE ARTIFACT rpms.tar.gz AS LOCAL centos-7.9-k3s-selinux-rpms.tar.gz

centos8-k3s-selinux-rpms:
  FROM centos:8.3.2011

  RUN yum install yum-utils -y
  WORKDIR /rpms

  RUN echo $'\n\
  [rancher-k3s-common-stable]\n\
  name=Rancher K3s Common (stable)\n\
  baseurl=https://rpm.rancher.io/k3s/stable/common/centos/8/noarch\n\
  enabled=1\n\
  gpgcheck=1\n\
  gpgkey=https://rpm.rancher.io/public.key'\
  >> /etc/yum.repos.d/rancher-k3s-common.repo

  # RUN repoquery --requires --resolve --recursive k3s-selinux | xargs -r yumdownloader
  RUN yumdownloader --assumeyes --resolve --destdir=/rpms k3s-selinux

  WORKDIR /
  RUN tar -czvf rpms.tar.gz /rpms

  SAVE ARTIFACT rpms.tar.gz AS LOCAL centos-8.3-k3s-selinux-rpms.tar.gz



helm:
  FROM alpine/helm:3.5.3
  WORKDIR /src

  # RUN apk add bash findutils

  # COPY manifests/charts/ .
  # RUN mkdir charts && bash -c "find . -mindepth 1 -maxdepth 1 -type d -exec helm package "{}" -u -d "./charts/" \;"
  RUN mkdir charts

  # Temporary helm chart hosters
  RUN helm repo add twuni https://helm.twun.io && \
      helm fetch twuni/docker-registry -d ./charts

  # RUN helm repo add bitnami https://charts.bitnami.com/bitnami && \
      # helm fetch bitnami/metallb -d ./charts

  RUN helm repo add traefik https://helm.traefik.io/traefik && \
      helm fetch traefik/traefik -d ./charts

  # Temporary!!
  GIT CLONE --branch main https://repo1.dso.mil/platform-one/big-bang/apps/sandbox/git-server.git git-server
  RUN helm package git-server/chart -d ./charts

  SAVE ARTIFACT /src/charts

images:
  FROM golang:1.16.3-buster
  GIT CLONE --branch main https://github.com/google/go-containerregistry.git /go-containerregistry

  WORKDIR /go-containerregistry/cmd/crane

  RUN CGO_ENABLED=0 go build -o /usr/local/bin/crane main.go

  WORKDIR /archive

  # Using crane and saving images like this is a _temporary_ solution
  RUN crane pull registry:2.7.1 registry.tar && \
      crane pull plndr/kube-vip:0.3.3 kube-vip.tar && \
      crane pull traefik:2.4.8 traefik.tar && \
      crane pull registry.dso.mil/platform-one/big-bang/apps/sandbox/git-server:0.0.1 git-server.tar

  SAVE ARTIFACT /archive

k3s:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /k3s

  ARG K3S_VERSION="v1.18.17+k3s1"

  RUN mkdir -p k3s

  RUN curl -fL "https://get.k3s.io" -o "k3s/init-k3s.sh"

  RUN curl -fL "https://github.com/k3s-io/k3s/releases/download/$K3S_VERSION/{k3s,k3s-airgap-images-amd64.tar,k3s-images.txt,sha256sum-amd64.txt}" -o "k3s/#1" && \
      ( cd k3s || exit ; sha256sum -c sha256sum-amd64.txt )

  RUN rm -f k3s/*.txt

  SAVE ARTIFACT /k3s

build:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16

  WORKDIR /payload

  COPY src /payload

  COPY +k3s/k3s assets/
  COPY +helm/charts assets/charts
  COPY +images/archive assets/images
  COPY manifests/autodeploy assets/manifests

  # Move the k3s image into the right location
  RUN mv assets/k3s/k3s-airgap-images-amd64.tar assets/images/

  # List the built asset tree 
  RUN find . | sed -e "s/[^-][^\/]*\// |/g" -e "s/|\([^ ]\)/|-\1/"

  RUN go build -o shift-package main.go
  
  SAVE ARTIFACT shift-package AS LOCAL shift-package

# Earthfile

centos7:
  FROM centos:7
  WORKDIR /deps

  RUN echo $'\n\
  [rancher-k3s-common-stable]\n\
  name=Rancher K3s Common (stable)\n\
  baseurl=https://rpm.rancher.io/k3s/stable/common/centos/7/noarch\n\
  enabled=1\n\
  gpgcheck=1\n\
  gpgkey=https://rpm.rancher.io/public.key'\
  >> /etc/yum.repos.d/rancher-k3s-common.repo

  RUN yum install -y createrepo
  RUN yum install -y --enablerepo="rancher*" --installroot=/tmp/k3s-selinux --downloadonly --downloaddir $(pwd) --releasever=7 k3s-selinux
  RUN createrepo -v .

  SAVE ARTIFACT /deps

centos8:
  FROM centos:8
  WORKDIR /deps

  RUN echo $'\n\
  [rancher-k3s-common-stable]\n\
  name=Rancher K3s Common (stable)\n\
  baseurl=https://rpm.rancher.io/k3s/stable/common/centos/8/noarch\n\
  enabled=1\n\
  gpgcheck=1\n\
  gpgkey=https://rpm.rancher.io/public.key'\
  >> /etc/yum.repos.d/rancher-k3s-common.repo

  RUN yum install -y createrepo
  RUN yum install -y --enablerepo="rancher*" --installroot=/tmp/k3s-selinux --downloadonly --downloaddir $(pwd) --releasever=8 k3s-selinux
  RUN createrepo -v .

  SAVE ARTIFACT /deps

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

  RUN helm repo add bitnami https://charts.bitnami.com/bitnami && \
      helm fetch bitnami/metallb -d ./charts

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
  RUN crane pull registry:2.7.1 plndr/kube-vip:0.3.3 plndr/plndr-cloud-provider:0.1.5 registry.dso.mil/platform-one/big-bang/apps/sandbox/git-server:0.0.1 images.tar.gz

  SAVE ARTIFACT /archive

build:
  FROM debian:buster-slim
  WORKDIR /build

  RUN apt update -y && apt install -y curl bash zstd

  RUN curl -fL -o makeself.run https://github.com/megastep/makeself/releases/download/release-2.4.3/makeself-2.4.3.run && \
      chmod +x makeself.run && \
      ./makeself.run && \
      mv makeself-2.4.3/makeself.sh /usr/local/bin/makeself && \
      mv makeself-2.4.3/makeself-header.sh /usr/local/bin/

  # package k3s as a single RUN cmd to better leverage layer caching
  RUN mkdir -p payload/k3s payload/rancher/k3s/agent/images && \
      curl -fL https://get.k3s.io -o payload/k3s/init-k3s.sh && \
      curl -fL https://github.com/k3s-io/k3s/releases/download/v1.20.4+k3s1/{k3s,k3s-airgap-images-amd64.tar,k3s-images.txt,sha256sum-amd64.txt} -o "payload/k3s/#1" && \
      ( cd payload/k3s || exit ; sha256sum -c sha256sum-amd64.txt ) && \
      mv payload/k3s/k3s-airgap-images-amd64.tar payload/rancher/k3s/agent/images

  # TODO: k3s-selinux
  # COPY +centos7/deps payload/rpms/centos7
  # COPY +centos8/deps payload/rpms/centos8
  COPY +helm/charts payload/rancher/k3s/server/static/charts
  COPY +images/archive payload/rancher/k3s/agent/images

  COPY k3s-config.yaml payload/k3s-config.yaml
  COPY manifests/autodeploy payload/rancher/k3s/server/manifests
  COPY install.sh payload/install.sh

  RUN makeself --zstd --sha256 payload bigbang-utility.run.zstd "BigBang Airgap Utility" ./install.sh

  SAVE ARTIFACT bigbang-utility.run.zstd AS LOCAL bigbang-utility.run.zstd


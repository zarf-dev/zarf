# Earthfile

ARG K3S_VERSION="v1.21.0+k3s1"

ARG REGISTRY_HELM_VERSION="1.10.1"
ARG GITEA_HELM_VERSION="2.2.5"

# Switch to IB images when ready
ARG APP_IMAGES="registry:2.7.1 gitea/gitea:1.13.7"

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

  RUN mkdir charts

  RUN helm repo add twuni https://helm.twun.io && \
      helm fetch twuni/docker-registry -d ./charts --version $REGISTRY_HELM_VERSION

  RUN helm repo add gitea-charts https://dl.gitea.io/charts/ && \
      helm fetch gitea-charts/gitea -d ./charts --version $GITEA_HELM_VERSION

  SAVE ARTIFACT /src/charts

k3s:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /downloads

  RUN curl -fL "https://get.k3s.io" -o "init-k3s.sh"

  RUN curl -fL "https://github.com/k3s-io/k3s/releases/download/$K3S_VERSION/{k3s,k3s-images.txt,sha256sum-amd64.txt}" -o "#1" && \
      sha256sum -c --ignore-missing "sha256sum-amd64.txt"

  SAVE ARTIFACT /downloads

images:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16
  GIT CLONE --branch main https://github.com/google/go-containerregistry.git /go-containerregistry
  WORKDIR /go-containerregistry/cmd/crane

  COPY +k3s/downloads/k3s-images.txt k3s-images.txt

  RUN k3s_images=$(cat "k3s-images.txt" | tr "\n" " ") && \
      images="$APP_IMAGES $k3s_images" && \
      echo "Cloning: $images" | tr " " "\n " && \
      go run main.go pull $images /go/images.tar

  SAVE ARTIFACT /go/images.tar

compress: 
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /payload

  RUN yum install -y zstd

  COPY manifests manifests

  # Pull in artifacts from other build stages
  COPY +k3s/downloads bin
  COPY +helm/charts charts
  COPY +images/images.tar images/images.tar

  # Create tarball of images
  RUN rm -f bin/*.txt 

  RUN tar -cv . | zstd -T0 -16 -f --long=25 - -o /export.tar.zst

  SAVE ARTIFACT /export.tar.zst

build:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16
  WORKDIR /payload

  # Pull in local assets
  COPY src .
  COPY +compress/export.tar.zst shift-package.tar.zst

  # Cache dep loading
  RUN go mod download 

  # Compute a shasum of the package tarball and inject at compile time
  RUN checksum=$(go run main.go checksum -f shift-package.tar.zst) && \
      echo "Computed tarball checksum: $checksum" && \
      go build -o shift-package -ldflags "-X shift/internal/utils.packageChecksum=$checksum" main.go

  # Validate the shasum before final packaging
  RUN ./shift-package validate

  SAVE ARTIFACT shift-package* AS LOCAL ./build/

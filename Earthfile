# Earthfile

ARG CONFIG="config.yaml"
ARG RHEL="false"
ARG DEV=true

# `WORKDIR=$PWD earthly shift-pak/+boilerplate` to setup the basic file structure
boilerplate:
  LOCALLY

  RUN git clone --depth 1 --branch master https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli.git repo && \
      cp -R repo/{payload,config.yaml,README.md} $WORKDIR/
    
clean-build:
  LOCALLY
  RUN rm -fr ${WORKDIR:-$PWD}/build

clone:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16

  RUN git clone --depth 1 --branch master https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli.git repo

  SAVE ARTIFACT repo

# Used to load the RHEL7 RPMS
rhel-rpms:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi$RHEL
  WORKDIR /rpms

  RUN --secret RHEL_USER=+secrets/RHEL_USER --secret RHEL_PASS=+secrets/RHEL_PASS \
      subscription-manager register --auto-attach --username=$RHEL_USER --password=$RHEL_PASS
  
  RUN subscription-manager repos --enable=rhel-$RHEL-server-extras-rpms

  RUN yumdownloader --resolve --destdir=/rpms/ container-selinux

  # Download the K3S SELinux RPM 
  RUN curl -L "https://github.com/k3s-io/k3s-selinux/releases/download/v0.3.stable.0/k3s-selinux-0.3-0.el7.noarch.rpm" -o "/rpms/k3s-selinux.rpm"

  SAVE ARTIFACT /rpms

helm:
  FROM alpine/helm:3.5.3
  SAVE ARTIFACT /usr/bin/helm

yq:
  FROM  mikefarah/yq
  SAVE ARTIFACT /usr/bin/yq

charts:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8

  WORKDIR /src 

  COPY +yq/yq /usr/bin
  COPY +helm/helm /usr/bin
  COPY $CONFIG .

  RUN mkdir charts

  RUN yq e '.charts[] | .name + " " + .url' $CONFIG | \
      while read line ; do echo "repo add $line" | xargs -t helm; done

  RUN yq e '.charts[] | .name + "/" + .name + " -d ./charts --version " + .version' $CONFIG | \
      while read line ; do echo "pull $line" | xargs -t helm; done

  SAVE ARTIFACT /src/charts

k3s:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /downloads

  COPY +yq/yq /usr/bin
  COPY $CONFIG /tmp/config.yaml

  RUN curl -fL "https://get.k3s.io" -o "init-k3s.sh"

  RUN K3S_VERSION=$(yq e '.k3s.version' /tmp/config.yaml) && \
      curl -fL "https://github.com/k3s-io/k3s/releases/download/$K3S_VERSION/{k3s,k3s-images.txt,sha256sum-amd64.txt}" -o "#1" && \
      sha256sum -c --ignore-missing "sha256sum-amd64.txt"

  SAVE ARTIFACT /downloads

images:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16
  GIT CLONE --branch main https://github.com/google/go-containerregistry.git /go-containerregistry
  WORKDIR /go-containerregistry/cmd/crane

  COPY +yq/yq /usr/bin
  COPY $CONFIG .
  COPY +k3s/downloads/k3s-images.txt k3s-images.txt

  RUN k3s_images=$(cat "k3s-images.txt" | tr "\n" " ") && \
      app_images=$(yq e '.images | join(" ")' $CONFIG) && \
      images="$app_images $k3s_images" && \
      echo "Cloning: $images" | tr " " "\n " && \
      go run main.go pull $images /go/images.tar

  SAVE ARTIFACT /go/images.tar

compress: 
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /payload

  # Allow custom build steps to run prior to tarball building
  BUILD ./payload/builder+build

  RUN yum install -y zstd

  # Pull in local resources
  COPY payload/bin bin
  COPY payload/manifests manifests
  COPY payload/misc misc

  # Pull in artifacts from other build stages
  COPY +k3s/downloads bin
  COPY +charts/charts charts
  COPY +images/images.tar images/images.tar

  # Optional include RHEL rpm build step
  IF [ $RHEL != "false" ]
    COPY +rhel-rpms/rpms rpms
  END

  # Quick housekeeping
  RUN rm -f bin/*.txt && mkdir -p rpms

  # Compress the tarball
  RUN tar -cv . | zstd -T0 -16 -f --long=25 - -o /export.tar.zst

  SAVE ARTIFACT /export.tar.zst


  
build:
  FROM registry1.dso.mil/ironbank/google/golang/golang-1.16
  WORKDIR /payload

  # Fix earthly conditional output for stupid IB permissions....
  USER 0
  RUN chown 1001 /run
  USER 1001

  # If dev mode use local src folder, otherwise go fetch it
  IF $DEV
    COPY src .
  ELSE
    COPY +clone/src .
  END

  COPY +compress/export.tar.zst shift-pack.tar.zst

  # Cache dep loading
  RUN go mod download 

  # Compute a shasum of the pack tarball and inject at compile time
  RUN checksum=$(go run main.go checksum -f shift-pack.tar.zst) && \
      echo "Computed tarball checksum: $checksum" && \
      go build -o shift-pack -ldflags \
      "-X repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/utils.packageChecksum=$checksum" main.go

  # Validate the shasum before final packaging
  RUN ./shift-pack validate
  RUN ls -lah shift-pack*

  BUILD +clean-build
  SAVE ARTIFACT shift-pack* AS LOCAL ./build/

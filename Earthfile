# Earthfile

ARG CONFIG="config.yaml"

# Test the deployment with vagrant, default to ubuntu.  Usage: `OS=rhel7 earthly +test`
test:
  LOCALLY
  RUN _os="${OS:=ubuntu}" vagrant destroy -f && vagrant up --no-color $OS && \
      echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh $OS\"\n\n\n"

test-destroy:
  LOCALLY
  RUN vagrant destroy -f

# Copy the helm 3 binary
helm:
  FROM alpine/helm:3.5.3
  SAVE ARTIFACT /usr/bin/helm

# Copy the yq 4 binary
yq:
  FROM mikefarah/yq
  SAVE ARTIFACT /usr/bin/yq

# Bring the zarf build artifact in
zarf:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  COPY ./cli+build/zarf zarf
  SAVE ARTIFACT zarf

# The baseline image with common binaries and $CONFIG
common:
  FROM registry1.dso.mil/ironbank/redhat/ubi/ubi8
  WORKDIR /payload

  COPY +helm/helm /usr/bin
  COPY +yq/yq /usr/bin
  COPY $CONFIG .

# Fetch the helm charts specified in $CONFIG 
charts:
  FROM +common

  RUN mkdir charts

  RUN yq e '.charts[] | .name + " " + .url' $CONFIG | \
      while read line ; do echo "repo add $line" | xargs -t helm; done

  RUN yq e '.charts[] | .name + "/" + .name + " -d ./charts --version " + .version' $CONFIG | \
      while read line ; do echo "pull $line" | xargs -t helm; done

  SAVE ARTIFACT charts

# Compress all assets in a single tar.zst file
compress: 
  FROM +common

  # Pull in artifacts from other build stages
  COPY +charts/charts charts

  COPY +zarf/zarf .

  # Compress the tarball
  RUN ./zarf archiver compress . /export.tar.zst

  SAVE ARTIFACT /export.tar.zst
  
# Final packaging of the binary/tarball/checksum assets
build:
  FROM +common

  COPY +zarf/zarf .

  # Copy the final compressed tarball for shasum / export
  COPY +compress/export.tar.zst zarf-initialize.tar.zst

  RUN sha256sum -b zarf* > zarf.sha256

  RUN ls -lah zarf*

  SAVE ARTIFACT zarf* AS LOCAL ./build/

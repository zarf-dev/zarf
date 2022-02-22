#!/bin/bash

# Copyright 2021 Defense Unicorns
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The install script is based off of the Apache-licensed script from Helm,
# the package manager for Kubernetes: https://github.com/helm/helm/blob/main/scripts/get-helm-3

: ${USE_SUDO:="true"}
: ${DEBUG:="false"}
: ${VERIFY_CHECKSUM:="true"}
: ${ZARF_INSTALL_DIR:="/usr/local/bin"}

HAS_CURL="$(type "curl" &> /dev/null && echo true || echo false)"
HAS_WGET="$(type "wget" &> /dev/null && echo true || echo false)"
HAS_OPENSSL="$(type "openssl" &> /dev/null && echo true || echo false)"

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
    aarch64) ARCH="arm64";;
    x86_64) ARCH="amd64";;
  esac
}

# initOS discovers the operating system for this system.
initOS() {
  OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')
}

#initBinary discovers which binary to download for this system based on the operating system and architecture.
initBinary() {
if [[ "$OS-$ARCH" = "darwin-arm64" ]]; then 
    : ${ZARF_BINARY:="zarf-mac-apple"}

elif [[ "$OS-$ARCH" = "darwin-amd64" ]]; then
    : ${ZARF_BINARY:="zarf-mac-intel"}

elif [[ "$OS-$ARCH" = "linux-amd64" ]] || [[ "$OS-$ARCH" = "linux-arm64" ]]; then 
    : ${ZARF_BINARY:="zarf"}
fi
}

# runs the given command as root (detects if we are root already)
runAsRoot() {
  if [ $EUID -ne 0 -a "$USE_SUDO" = "true" ]; then
    sudo "${@}"
  else
    "${@}"
  fi
}

# verifySupported checks that the os/arch combination is supported for
# binary builds, as well whether or not necessary tools are present.
verifySupported() {
  local supported="darwin-amd64\ndarwin-arm64\nlinux-amd64\nlinux-arm64"
  if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
    echo "No prebuilt binary for ${OS}-${ARCH}."
    echo "To build from source, go to https://github.com/defenseunicorns/zarf"
    exit 1
  fi

  if [ "${HAS_CURL}" != "true" ] && [ "${HAS_WGET}" != "true" ]; then
    echo "Either curl or wget is required"
    exit 1
  fi

  if [ "${VERIFY_CHECKSUM}" == "true" ] && [ "${HAS_OPENSSL}" != "true" ]; then
    echo "In order to verify checksum, openssl must first be installed."
    echo "Please install openssl or set VERIFY_CHECKSUM=false in your environment."
    exit 1
  fi
}

# checkDesiredVersion checks if the desired version is available.
checkDesiredVersion() {
  if [ "x$DESIRED_VERSION" == "x" ]; then
    # Get tag from release URL
    local latest_release_url="https://github.com/defenseunicorns/zarf/releases"
    if [ "${HAS_CURL}" == "true" ]; then
      TAG=$(curl -Ls $latest_release_url | grep 'href="/defenseunicorns/zarf/releases/tag/v0.[0-15]*.[0-9]*\"' | sed -E 's/.*\/defenseunicorns\/zarf\/releases\/tag\/(v[0-9\.]+)".*/\1/g' | head -1)
    elif [ "${HAS_WGET}" == "true" ]; then
      TAG=$(wget $latest_release_url -O - 2>&1 | grep 'href="/defenseunicorns/zarf/releases/tag/v0.[0-15]*.[0-9]*\"' | sed -E 's/.*\/defenseunicorns\/zarf\/releases\/tag\/(v[0-9\.]+)".*/\1/g' | head -1)
    fi
  else
    TAG=$DESIRED_VERSION
  fi
}

# checkZarfInstalledVersion checks if a Zarf binary is already installed and
# removes it prior to installing 
checkZarfInstalled() {
  if [[ -f $(which "$ZARF_BINARY") ]]; then
    runAsRoot rm $(which "$ZARF_BINARY")
  fi
}

# downloadFile downloads the latest binary and also the checksum
# for that binary.
downloadFile() {
  DOWNLOAD_URL="https://zarf-public.s3-us-gov-west-1.amazonaws.com/release/$TAG/$ZARF_BINARY"
  CHECKSUM_URL="https://zarf-public.s3-us-gov-west-1.amazonaws.com/release/$TAG/zarf.sha256"
  ZARF_TMP_ROOT="$(mktemp -dt zarf-installer-XXXXXX)"
  ZARF_SUM_FILE="$ZARF_TMP_ROOT/zarf.sha256"

  echo "Downloading $DOWNLOAD_URL"
  if [ "${HAS_CURL}" == "true" ]; then
    curl -SsL "$CHECKSUM_URL" -o "$ZARF_SUM_FILE"
    curl -Ssl "$DOWNLOAD_URL" -o "$ZARF_TMP_ROOT/$ZARF_BINARY"
  elif [ "${HAS_WGET}" == "true" ]; then
    wget -q -O "$ZARF_SUM_FILE" "$CHECKSUM_URL"
    wget -q -O "$ZARF_TMP_ROOT/$ZARF_BINARY" "$DOWNLOAD_URL"
  fi
}

# verifyFile verifies the SHA256 checksum of the binary package
verifyFile() {
  if [ "${VERIFY_CHECKSUM}" == "true" ]; then
    verifyChecksum
  fi
}

# installFile installs the zarf binary.
installFile() {
  ZARF_TMP="$ZARF_TMP_ROOT/$ZARF_BINARY"
  mkdir -p "$ZARF_TMP_ROOT"
  echo "Preparing to install $ZARF_BINARY into ${ZARF_INSTALL_DIR}"
  runAsRoot cp "$ZARF_TMP" "$ZARF_INSTALL_DIR/$ZARF_BINARY"
  runAsRoot chmod 755 "$ZARF_INSTALL_DIR/$ZARF_BINARY"
  echo "$ZARF_BINARY installed into $ZARF_INSTALL_DIR/$ZARF_BINARY"
}

# verifyChecksum verifies the SHA256 checksum of the binary package.
verifyChecksum() {
  printf "Verifying checksum... "
  local sum=$(openssl sha1 -sha256 $ZARF_TMP_ROOT/$ZARF_BINARY | awk '{print $2}')
  local expected_sum=$(cat $ZARF_SUM_FILE | awk '{print $1}')
  if [[ ! "${expected_sum}" =~ "$sum" ]]; then
    echo "SHA sum of Zarf binary does not match. Aborting."
    exit 1
  elif [[ "${expected_sum}" =~ "$sum" ]]; then
    echo "SHA sum of Zarf binary matches."
  fi
  echo "Done."
}

# fail_trap is executed if an error occurs.
fail_trap() {
  result=$?
  if [ "$result" != "0" ]; then
    if [[ -n "$INPUT_ARGUMENTS" ]]; then
      echo "Failed to install $ZARF_BINARY with the arguments provided: $INPUT_ARGUMENTS"
      help
    else
      echo "Failed to install $ZARF_BINARY"
    fi
    echo -e "\tFor support, go to https://github.com/defenseunicorns/zarf."
  fi
  cleanup
  exit $result
}

# testVersion tests the installed client to make sure it is working.
testVersion() {
  set +e
  zarf="$(command -v $ZARF_BINARY)"
  if [ "$?" = "1" ]; then
    echo "$ZARF_BINARY not found. Is $ZARF_INSTALL_DIR on your "'$PATH?'
    exit 1
  fi
  set -e
}

# help provides possible cli installation arguments
help () {
  echo "Accepted cli arguments are:"
  echo -e "\t[--help|-h ] ->> prints this help"
  echo -e "\t[--version|-v <desired_version>] . When not defined it fetches the latest release from GitHub"
  echo -e "\te.g. --version v0.15.0 or -v canary"
  echo -e "\t[--no-sudo]  ->> install without sudo"
}

# cleanup temporary files
cleanup() {
  if [[ -d "${ZARF_TMP_ROOT:-}" ]]; then
    rm -rf "$ZARF_TMP_ROOT"
  fi
  
  if [[ -f "get_zarf.sh" ]]; then
    rm "get_zarf.sh"
  fi 
}

# Execution

#Stop execution on any error
trap "fail_trap" EXIT
set -e

# Set debug if desired
if [ "${DEBUG}" == "true" ]; then
  set -x
fi

# Parsing input arguments (if any)
export INPUT_ARGUMENTS="${@}"
set -u
while [[ $# -gt 0 ]]; do
  case $1 in
    '--version'|-v)
       shift
       if [[ $# -ne 0 ]]; then
           export DESIRED_VERSION="${1}"
       else
           echo -e "Please provide the desired version. e.g. --version v0.15.0 or -v canary"
           exit 0
       fi
       ;;
    '--no-sudo')
       USE_SUDO="false"
       ;;
    '--help'|-h)
       help
       exit 0
       ;;
    *) exit 1
       ;;
  esac
  shift
done
set +u

initArch
initOS
initBinary
verifySupported
checkDesiredVersion
checkZarfInstalled
downloadFile
verifyFile
installFile
testVersion
cleanup
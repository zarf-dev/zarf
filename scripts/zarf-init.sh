#!/bin/bash
set -eo pipefail

show_usage() {
  echo -e 'Usage:'
  echo "$0 [-h|--help] [--zarfversion <version>] [--components <zarfcomponents>]"
  echo ''
  echo ' Pre-requisites '
  echo '  - curl'
  echo '  - shasum'
  echo ' '
  echo ' Arguments '
  echo ' '
  echo ' -h|--help      : Show this usage.'
  echo ' '
  echo ' --zarfversion  : Zarf tag version to use. This script has been tested with 0.17 and 0.18 versions.'
  echo '                  Default: latest.'
  echo ' --components   : Zarf optional components to install. Note: Version 0.18 changes the name of some components.'
  echo '                  Default: git-server.'
}

# Check the OS and architecture
ZARF_OS=`uname` # Options: Darwin (Mac), Linux
ZARF_ARCH=`uname -p` # On Mac returns i386 and on Linux x86_64
ZARF_REPO='defenseunicorns/zarf'
ZARF_COMPONENTS='git-server'
PARAMS=""
while (( $# )); do
  case "$1" in
    -h|--help)
      show_usage
      exit 0
      ;;
    --zarfversion)
      ZARF_VERSION=$2
      shift 2
      ;;
    --components)
      ZARF_COMPONENTS=$2
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*|--*)
      echo -e "Unsupported flag $1\n" >&2
      show_usage
      exit 1
      ;;
    *)
      PARAMS="$PARAMS $1"
      shift
      ;;
  esac
done



if [ -z "$ZARF_VERSION" ]
then
  ZARF_VERSION=`curl -s -L "https://api.github.com/repos/${ZARF_REPO}/tags" | jq -r '.[0].name'`
fi

# If we could not get a version from curl then we stick to 0.19.4
if [[ -z "$ZARF_VERSION" ]]; then
  ZARF_VERSION="v0.19.4"
fi

echo "------------------------"
echo "Attemping to download zarf version ${ZARF_VERSION} for ${ZARF_OS} ${ZARF_ARCH}"
echo "------------------------"

# There were multiple changes on versions that I am not sure we want to support:
#
# < v0.19.1 files were named zarf (linux), zarf-mac-intel and zarf-mac-apple, and zarf.sha256
# v0.19.2 files were named zarf_0.19.2_* without the v of version, and zarf.sha256
# > v0.19.3 files are named zarf_v0.19.3_* with the v of version, and checksums.txt

if [[ $ZARF_OS == 'Linux' ]]; then
  ZARF_FILE="zarf_${ZARF_VERSION}_Linux_amd64"
else
  if [[ $ZARF_OS == 'Darwin' ]]; then
    if [[ $ZARF_ARCH == "i386" ]]; then
      ZARF_FILE="zarf_${ZARF_VERSION}_Darwin_amd64"
    else
      ZARF_FILE="zarf_${ZARF_VERSION}_Darwin_arm64"
    fi
  else
    echo "ERROR: Error downloading zarf binary, unrecognized OS, please install zarf manually from https://github.com/defenseunicorns/zarf"
    exit 1
  fi
fi

ZARF_BUNDLE_FILENAME="zarf-init-amd64.tar.zst"
ZARF_CHECKSUM_FILENAME="checksums.txt"
ZARF_BINARY_URL="https://github.com/${ZARF_REPO}/releases/download/${ZARF_VERSION}/${ZARF_FILE}"
ZARF_BUNDLE_URL="https://github.com/${ZARF_REPO}/releases/download/${ZARF_VERSION}/${ZARF_BUNDLE_FILENAME}"
ZARF_CHECKSUM_URL="https://github.com/${ZARF_REPO}/releases/download/${ZARF_VERSION}/${ZARF_CHECKSUM_FILENAME}"
TMP_DIR="${HOME}/.zarf"

echo "Zarf working directory: ${TMP_DIR}"

mkdir -p ${TMP_DIR}
pushd ${TMP_DIR} > /dev/null

# Based on OS and Arch download the proper binary and bundle

if [[ -f ${TMP_DIR}/${ZARF_FILE} ]]; then
  echo "Zarf binary ${ZARF_FILE} already exists, skipping download"
else
  echo "Downloading zarf binary ${ZARF_FILE}"

  curl -s -L -o ${TMP_DIR}/${ZARF_FILE} ${ZARF_BINARY_URL}
  ln -fs ${TMP_DIR}/${ZARF_FILE} ${TMP_DIR}/zarf
fi

# For now we always download because there is no easy way to tell the version based in
# the filename, we could potentially use zarf package inspect and look into the last line:
#          The package was built with Zarf CLI version v0.19.4
# But that will require enable execution permissions on the zarf binary, and executing inspect in the package
# before verifing checksums which is potentially dangerous.
# Another way is to create a "version file", we cannot rename the bundle output because then we need to rename the filename
# into the checksums.txt, but we want to keep it as clean as possible

#if [[ -f ${TMP_DIR}/${ZARF_BUNDLE_FILENAME} ]]; then
#  echo "Zarf package ${ZARF_BUNDLE_FILENAME} already exists, skipping download"
#else
  echo "Downloading zarf package init bundle"

  curl -s -L -o ${TMP_DIR}/${ZARF_BUNDLE_FILENAME} ${ZARF_BUNDLE_URL}
#fi

# Download the checksum file, and verify that is the right binary

echo "Verifying checksums of files"

curl -s -L -o ${TMP_DIR}/${ZARF_CHECKSUM_FILENAME} ${ZARF_CHECKSUM_URL}
shasum --ignore-missing -c ${ZARF_CHECKSUM_FILENAME}

# Init Zarf 

echo "Initializing zarf in the cluster"

chmod +x ${ZARF_FILE}
./${ZARF_FILE} init --components ${ZARF_COMPONENTS} --confirm

popd > /dev/null
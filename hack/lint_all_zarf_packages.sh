#!/bin/bash

use_build=$1
BASEDIR=$(dirname "$0")
cd ../$BASEDIR
find "." -type f -name 'zarf.yaml' | while read -r yaml_file; do
  dir=$(dirname "$yaml_file")
  echo "Running 'zarf prepare lint' in directory: $dir"
  if [ "$use_build" = true ]; then
    ./build/zarf prepare lint $dir
  else
    zarf prepare lint $dir
  fi
  echo "---"
done

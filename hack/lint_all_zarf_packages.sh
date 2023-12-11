#!/bin/bash

use_build=$1
SCRIPT=$(realpath "$0")
SCRIPTPATH=$(dirname "$SCRIPT")
cd $SCRIPTPATH
cd ..
find "." -type f -name 'zarf.yaml' | while read -r yaml_file; do
  dir=$(dirname "$yaml_file")
  if [[ "$dir" == *src/test/* ]] && [ "$use_build" != true ]; then
      continue
  fi
  echo "Running 'zarf prepare lint' in directory: $dir"
  if [ "$use_build" = true ]; then
    ./build/zarf prepare lint $dir
  else
    zarf prepare lint $dir
  fi
  echo "---"
done

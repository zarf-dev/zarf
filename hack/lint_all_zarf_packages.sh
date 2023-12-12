#!/bin/bash

lint_src_test=$1
SCRIPT=$(realpath "$0")
SCRIPTPATH=$(dirname "$SCRIPT")
cd $SCRIPTPATH
cd ..
find "." -type f -name 'zarf.yaml' | while read -r yaml_file; do
  dir=$(dirname "$yaml_file")
  if [[ "$dir" == *src/test/* ]] && [ "$lint_src_test" != true ]; then
      continue
  fi
  echo "Running 'zarf prepare lint' in directory: $dir"
  ./build/zarf prepare lint $dir
  echo "---"
done

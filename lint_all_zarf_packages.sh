#!/bin/bash

find "." -type f -name 'zarf.yaml' | while read -r yaml_file; do
  dir=$(dirname "$yaml_file")
  echo "Running 'zarf prepare lint' in directory: $dir"
  (cd "$dir" && ~/code/zarf/build/zarf prepare lint)
  echo "---"
done

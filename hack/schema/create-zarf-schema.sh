#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

cd $SCRIPT_DIR

add_yaml_extensions() {
  local src=$1
  local dst=$2

  jq '
    def addPatternProperties:
      . +
      if has("properties") then
        {"patternProperties": {"^x-": {}}}
      else
        {}
      end;

    walk(if type == "object" then addPatternProperties else . end)
  ' "$src" > "$dst"
}

go run main.go > "zarf_package_v1alpha1.schema.json"

add_yaml_extensions "zarf_package_v1alpha1.schema.json" "../../zarf.schema.json"

rm zarf_package_v1alpha1.schema.json

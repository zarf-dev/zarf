#!/usr/bin/env bash

set -euo pipefail

# Create the json schema for the zarf.yaml
go run main.go internal gen-config-schema > zarf.schema.json

# Adds pattern properties to all definitions to allow for yaml extensions
jq '
  def addPatternProperties:
    . +
    if type == "object" and has("properties") then
      {"patternProperties": {"^x-": {}}}
    else
      {}
    end;

  walk(if type == "object" then addPatternProperties else . end)
' zarf.schema.json > temp_zarf.schema.json

mv temp_zarf.schema.json zarf.schema.json

#!/usr/bin/env sh

# Create the json schema for the zarf.yaml
go run main.go internal gen-config-schema > zarf.schema.json

# Adds pattern properties to all definitions to allow for yaml extensions
jq '.definitions |= map_values(. + {"patternProperties": {"^x-": {}}})' zarf.schema.json > temp_zarf.schema.json
mv temp_zarf.schema.json zarf.schema.json

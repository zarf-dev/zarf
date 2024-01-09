#!/usr/bin/env sh

# Create the json schema for the zarf.yaml
go run main.go internal gen-config-schema > zarf.schema.json

# Adds pattern properties to all definitions to allow for yaml extensions
jq '.definitions |= map_values(. + {"patternProperties": {"^x-": {}}})' zarf.schema.json > temp_zarf.schema.json
mv temp_zarf.schema.json zarf.schema.json

# Create docs from the zarf.yaml JSON schema
docker run -v $(pwd):/app -w /app --rm python:3.8-alpine /bin/sh -c "pip install json-schema-for-humans && generate-schema-doc --config-file hack/.templates/jsfh-config.json zarf.schema.json docs/3-create-a-zarf-package/4-zarf-schema.md"

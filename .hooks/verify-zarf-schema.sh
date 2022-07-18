#!/usr/bin/env sh
go run main.go internal config-schema > zarf.schema.json
docker run -it -v $(pwd):/app -w /app --rm python:3.8-alpine /bin/sh -c "pip install json-schema-for-humans && generate-schema-doc --config-file .hooks/jsfh-config.json zarf.schema.json docs/4-user-guide/3-zarf-schema.md"

# schema generation

This go project generates the JSON schema for zarf.yaml files.

## Usage
This code should be called with `./create-zarf-schema.sh` which will generate all of the schemas, add yaml extension, and move the schema files to their proper place in the repo.

Alternatively run `go run main.go` to print the json schema to the stdout.

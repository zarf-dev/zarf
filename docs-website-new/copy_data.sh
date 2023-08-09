#!/usr/bin/env bash

cp ../zarf.schema.json data/zarf_schema.json


includePath="static/includes"
mkdir -p ${includePath}
cp ../CONTRIBUTING.md ${includePath}/CONTRIBUTING.md

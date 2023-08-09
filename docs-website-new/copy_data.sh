#!/usr/bin/env bash

cp ../zarf.schema.json data/zarf_schema.json


includePath="static/includes"
imagePath="assets/img"
mkdir -p ${includePath}
cp ../CONTRIBUTING.md ${includePath}/CONTRIBUTING.md
cp ../docs/.images/architecture.drawio.svg ${includePath}/architecture.drawio.svg

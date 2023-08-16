#!/usr/bin/env bash

cp ../zarf.schema.json data/zarf_schema.json


includePath="static/includes"
imagePath="assets/img"
mkdir -p ${includePath}
cp ../CONTRIBUTING.md ${includePath}/CONTRIBUTING.md
cp ../docs/.images/architecture.drawio.svg ${includePath}/architecture.drawio.svg

hugodocs

## One time conversion
mkdir -p ../docs-hugo/docs
cp -r ../docs/examples ../docs-hugo/docs
mv ../docs ../docs-old
mv ../docs-hugo/docs ..
rm -rf ../docs-hugo

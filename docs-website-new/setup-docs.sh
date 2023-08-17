#!/usr/bin/env bash

includePath="static/includes"
mkdir -p ${includePath}
cp ../CONTRIBUTING.md ${includePath}/CONTRIBUTING.md
cp ../.images/architecture.drawio.svg ${includePath}/architecture.drawio.svg
cp ../zarf.schema.json data/zarf_schema.json



# This converts the docs from the current format to work with Hugo. After conversion, the docs gene4ration scripts
# should be modified to create the new format.
hugodocs


rm -rf ../docs-old && \
mkdir -p ../docs-hugo/docs && \
cp -R ../docs/examples ../docs-hugo/docs && \
mv ../docs ../docs-old && \
mv ../docs-hugo/docs .. && \
rm -rf ../docs-hugo

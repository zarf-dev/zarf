# SBOMs in Zarf

A Zarf generates a Software Bill of Material (SBOM) for each of the images within a Zarf package.  This allows consumers of Zarf packages to get in depth knowledge of what is contained within the Zarf package.

## What are SBOMs?

SBOMs are a collection of dependencies, tools, and other information about how a piece of software was built.  Zarf collects information about the images within a Zarf package such as the base distro, packages installed, licenses of installed software, and more.

This allows users of the software to have a clearer understanding of what is running and find potential vulnerabilities that may otherwise go undetected.


## How does Zarf generate SBOMs?

Zarf uses [Syft](https://github.com/anchore/syft/) to generate SBOMs for each image during `zarf package create`

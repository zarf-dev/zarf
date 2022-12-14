# 8. Begin using unit tests

Date: 2022-12-13

## Status

Accepted

## Context

Images are not the only way that dependencies can be brought in via a Zarf package (our own init package has two components that are not images yet could have vulnerabilities within them, k3s and the injector).  We should SBOM these in addition to images so that we are providing a more complete picture to our users.

Potential considerations:

1. Run Syft against the entire zarf temp build directory
    Pros:
        - This would provide the most information for what _might_ be in a Zarf package
    Cons:
        - If git repos are brought in you could catch many index files that are actually not in the package but are dependencies of the repos leading to confusion

2. Run Syft against files and dataInjections
    Pros:
        - We know these files are actually inside of the package and won't just be noise
        - This is the most common two ways for people to include addtional artifacts in packages (we use files in our own init package)
    Cons:
        - This is only a subset of what Zarf provides and someone could commit an artifact to a git repo for example

3. Allow user flexibility in what gets SBOMed
    Pros:
        - This provides the user the most flexibility
    Cons:
        - This could be complex to get right on implementation and if it were optional may be a forgotten feature by most users

## Decision

It was decided that we would SBOM the files and dataInjections inclusions by component (2) and include them into our SBOM viewer that way.  This will allow us to characterize what SBOMing in Zarf may look like going forward without introducing something that is just optional or that might make too much noise at first blush.

## Consequences

This will allow for more SBOM fidelity on the variety of ways users can bring dependencies into the airgap but will not catch everything.  We will need to closely look at upstream tooling like Syft to see how we can both improve it and use it better within Zarf (the inability to parse .whl files without renaming or unpacking is an example of this, and we may want to disable index parsers in the future so lots of tuning to come).

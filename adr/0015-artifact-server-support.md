# 12. Artifact Server Support

Date: 2023-01-30

## Status

Accepted

## Context

Zarf currently supports git servers and container registries within the airgap to host dependencies for applications, and while this works well for most production deployments, it doesn't cater as well to airgap development where you may need artifacts and libraries for various coding languages to compile and develop software.  Zarf's support of `git` is also somewhat lacking in that it only supports flux `GitRepository` objects and does not support more generic use cases where clients try to reach out to upstream `git` hosts in a more native way.

## Decision

Since we already had an artifact registry available to us in Gitea, it was decided to utilize that as the default provider for this functionality in addition to matching the external support for `git` and `registry` servers with a new `artifact` server specification on `init`.  From here, to access the configured server we had two main options:

1. Transform any artifact references statically (i.e. swap upstream URLs in build scripts before bringing them into the environment)
2. Transform any artifact references dynamically (i.e. swap upstream URLs in an active proxy that any build scripts could pickup through DNS)

It was decided to go with #2 since this would allow us to support a wider array of build technologies (including those that wrap existing commands like Buck/Bazel/Make) as well as support builds that may be coming from resources not brought in by Zarf.  This allows for more flexibility in how Zarf could transform URLs while allowing these commands to run as they would on the internet side without any modification.

## Consequences

For now this should be an internal-only feature while we work through issues and continue to experiment with this functionality given that it may be confusing to use or have rough edges for a while.  This also ties us to building specific http request matching logic for various artifact technologies which currently is done first via User Agent, then by parsing the URL to look for the artifact protocol specific path information.  While this works, it must be created per artifact technology and right not only supports git (http), pip, npm, and generic repositories and registries.

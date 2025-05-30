---
title: zarf package mirror-resources
description: Zarf CLI command reference for <code>zarf package mirror-resources</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

## zarf package mirror-resources

Mirrors a Zarf package's internal resources to specified image registries and git repositories

### Synopsis

Unpacks resources and dependencies from a Zarf package archive and mirrors them into the specified
image registries and git repositories within the target environment

```
zarf package mirror-resources [ PACKAGE_SOURCE ] [flags]
```

### Examples

```

# Mirror resources to internal Zarf resources - by default this will use Zarf state if available
$ zarf package mirror-resources <your-package.tar.zst>

# Mirror resources to external resources
$ zarf package mirror-resources <your-package.tar.zst> \
	--registry-url registry.enterprise.corp \
	--registry-push-username <registry-push-username> \
	--registry-push-password <registry-push-password> \
	--git-url https://git.enterprise.corp \
	--git-push-username <git-push-username> \
	--git-push-password <git-push-password>

# Mirroring resources can be specified by artifact type - this will only mirror images
$ zarf package mirror-resources <your-package.tar.zst> --images \
	--registry-url registry.enterprise.corp \
	--registry-push-username <registry-push-username> \
	--registry-push-password <registry-push-password>

# Mirroring for repositories can be specified by artifact type - this will only mirror git repositories
$ zarf package mirror-resources <your-package.tar.zst> --repos \
	--git-url https://git.enterprise.corp \
	--git-push-username <git-push-username> \
	--git-push-password <git-push-password>

```

### Options

```
      --components string               Comma-separated list of components to mirror.  This list will be respected regardless of a component's 'required' or 'default' status.  Globbing component names with '*' and deselecting components with a leading '-' are also supported.
      --confirm                         Confirms package deployment without prompting. ONLY use with packages you trust. Skips prompts to review SBOM, configure variables, select optional components and review potential breaking changes.
      --git-push-password string        Password for the push-user to access the git server
      --git-push-username string        Username to access to the git server Zarf is configured to use. User must be able to create repositories via 'git push' (default "zarf-git-user")
      --git-url string                  External git server url to use for this Zarf cluster
  -h, --help                            help for mirror-resources
      --images                          mirror only the images
      --no-img-checksum                 Turns off the addition of a checksum to image tags (as would be used by the Zarf Agent) while mirroring images.
      --registry-push-password string   Password for the push-user to connect to the registry
      --registry-push-username string   Username to access to the registry Zarf is configured to use (default "zarf-push")
      --registry-url string             External registry url address to use for this Zarf cluster
      --repos                           mirror only the git repositories
      --retries int                     Number of retries to perform for Zarf deploy operations like git/image pushes or Helm installs (default 3)
      --shasum string                   Shasum of the package to pull. Required if pulling a https package. A shasum can be retrieved using 'zarf dev sha256sum <url>'
      --skip-signature-validation       Skip validating the signature of the Zarf package
```

### Options inherited from parent commands

```
  -a, --architecture string        Architecture for OCI images and Zarf packages
      --insecure-skip-tls-verify   Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -k, --key string                 Path to public key file for validating signed packages
      --log-format string          Select a logging format. Defaults to 'console'. Valid options are: 'console', 'json', 'dev'. (default "console")
  -l, --log-level string           Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-color                   Disable terminal color codes in logging and stdout prints.
      --oci-concurrency int        Number of concurrent layer operations when pulling or pushing images or packages to/from OCI registries. (default 6)
      --plain-http                 Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.
      --tmpdir string              Specify the temporary directory to use for intermediate files
      --zarf-cache string          Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf package](/commands/zarf_package/)	 - Zarf package commands for creating, deploying, and inspecting packages


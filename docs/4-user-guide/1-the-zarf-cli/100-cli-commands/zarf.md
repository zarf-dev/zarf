## zarf

DevSecOps Airgap Toolkit

### Synopsis

Zarf is a toolkit for building and deploying airgapped Kubernetes clusters.

```
zarf [COMMAND] [flags]
```

### Options

```
  -a, --architecture string   Set the architecture to use for the package. Valid options are: amd64, arm64.
  -h, --help                  help for zarf
  -l, --log-level string      Set the log level. Valid options are: warn, info, debug, trace. (default "info")
      --no-log-file           Disable logging to a file.
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf completion](zarf_completion.md)	 - Generate the autocompletion script for the specified shell
* [zarf connect](zarf_connect.md)	 - Access services or pods deployed in the cluster.
* [zarf destroy](zarf_destroy.md)	 - Tear it all down, we'll miss you Zarf...
* [zarf init](zarf_init.md)	 - Prepares a k8s cluster for the deployment of Zarf packages
* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages
* [zarf prepare](zarf_prepare.md)	 - Tools to help prepare assets for packaging
* [zarf tools](zarf_tools.md)	 - Collection of additional tools to make airgap easier
* [zarf version](zarf_version.md)	 - SBOM tools provided by Anchore Syft


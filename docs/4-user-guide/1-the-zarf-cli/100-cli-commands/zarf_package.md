## zarf package

Zarf package commands for creating, deploying, and inspecting packages

### Options

```
  -h, --help   help for package
```

### Options inherited from parent commands

```
  -a, --architecture string   Set the architecture to use for the package. Valid options are: amd64, arm64.
  -l, --log-level string      Set the log level. Valid options are: warn, info, debug, trace. (default "info")
      --no-log-file           Disable logging to a file.
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf](zarf.md)	 - DevSecOps Airgap Toolkit
* [zarf package create](zarf_package_create.md)	 - Use to create a Zarf package from a given directory or the current directory
* [zarf package deploy](zarf_package_deploy.md)	 - Use to deploy a Zarf package from a local file or URL (runs offline)
* [zarf package inspect](zarf_package_inspect.md)	 - Lists the payload of a Zarf package (runs offline)
* [zarf package list](zarf_package_list.md)	 - List out all of the packages that have been deployed to the cluster
* [zarf package remove](zarf_package_remove.md)	 - Use to remove a Zarf package that has been deployed already


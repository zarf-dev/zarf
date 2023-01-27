## zarf tools

Collection of additional tools to make airgap easier

### Options

```
  -h, --help   help for tools
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
      --insecure              Allow access to insecure registries and disable other recommended security enforcements. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

### SEE ALSO

* [zarf](zarf.md)	 - DevSecOps for Airgap
* [zarf tools archiver](zarf_tools_archiver.md)	 - Compress/Decompress generic archives, including Zarf packages.
* [zarf tools clear-cache](zarf_tools_clear-cache.md)	 - Clears the configured git and image cache directory.
* [zarf tools gen-pki](zarf_tools_gen-pki.md)	 - Generates a Certificate Authority and PKI chain of trust for the given host
* [zarf tools get-git-password](zarf_tools_get-git-password.md)	 - Returns the push user's password for the Git server
* [zarf tools monitor](zarf_tools_monitor.md)	 - Launch a terminal UI to monitor the connected cluster using K9s.
* [zarf tools registry](zarf_tools_registry.md)	 - Tools for working with container registries using go-containertools.
* [zarf tools sbom](zarf_tools_sbom.md)	 - Generates a Software Bill of Materials (SBOM) for the given package


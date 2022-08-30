## zarf prepare

Tools to help prepare assets for packaging

### Options

```
  -h, --help   help for prepare
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
```

### SEE ALSO

* [zarf](zarf.md)	 - DevSecOps Airgap Toolkit
* [zarf prepare find-images](zarf_prepare_find-images.md)	 - Evaluates components in a zarf file to identify images specified in their helm charts and manifests
* [zarf prepare patch-git](zarf_prepare_patch-git.md)	 - Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE.  NOTE: 
This should only be used for manifests that are not mutated by the Zarf Agent Mutating Webhook.
* [zarf prepare sha256sum](zarf_prepare_sha256sum.md)	 - Generate a SHA256SUM for the given file


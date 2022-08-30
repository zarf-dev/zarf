## zarf init

Prepares a k8s cluster for the deployment of Zarf packages

### Synopsis

Injects a docker registry as well as other optional useful things (such as a git server and a logging stack) into a k8s cluster under the 'zarf' namespace to support future application deployments. 
If you do not have a k8s cluster already configured, this command will give you the ability to install a cluster locally.

This command looks for a zarf-init package in the local directory that the command was executed from. If no package is found in the local directory and the Zarf CLI exists somewhere outside of the current directory, Zarf will failover and attempt to find a zarf-init package in the directory that the Zarf binary is located in.


```
zarf init [flags]
```

### Options

```
      --components string      Comma-separated list of components to install.
      --confirm                Confirm the install without prompting
  -h, --help                   help for init
      --nodeport string        Nodeport to access the Zarf container registry. Between [30000-32767]
      --secret string          Root secret value that is used to 'seed' other secrets
      --storage-class string   Describe the StorageClass to be used
      --tmpdir string          Specify the temporary directory to use for intermediate files
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
```

### SEE ALSO

* [zarf](zarf.md)	 - DevSecOps Airgap Toolkit


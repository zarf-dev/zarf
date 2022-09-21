## zarf init

Prepares a k8s cluster for the deployment of Zarf packages

### Synopsis

Injects a docker registry as well as other optional useful things (such as a git server and a logging stack) into a k8s cluster under the 'zarf' namespace to support future application deployments. 
If you do not have a k8s cluster already configured, this command will give you the ability to install a cluster locally.

This command looks for a zarf-init package in the local directory that the command was executed from. If no package is found in the local directory and the Zarf CLI exists somewhere outside of the current directory, Zarf will failover and attempt to find a zarf-init package in the directory that the Zarf binary is located in.



Example Usage:
# Initializing without any optional components:
zarf init

# Initializing w/ Zarfs internal git server:
zarf init --components=git-server

# Initializing w/ Zarfs internal git server and PLG stack:
zarf init --components=git-server,logging

# Initializing w/ an internal registry but with a different nodeport:
zarf init --nodeport=30333

# Initializing w/ an external registry:
zarf init --registry-push-password={PASSWORD} --registry-push-username={USERNAME} --registry-url={URL}

# Initializing w/ an external git server:
zarf init --git-push-password={PASSWORD} --git-push-username={USERNAME} --git-url={URL}



```
zarf init [flags]
```

### Options

```
      --components string               Comma-separated list of components to install.
      --confirm                         Confirm the install without prompting
      --git-pull-password string        Password for the pull-only user to access the git server
      --git-pull-username string        Username for pull-only access to the git server
      --git-push-password string        Password for the push-user to access the git server
      --git-push-username string        Username to access to the git server Zarf is configured to use. User must be able to create repositories via 'git push' (default "zarf-git-user")
      --git-url string                  External git server url to use for this Zarf cluster
  -h, --help                            help for init
      --nodeport int                    Nodeport to access a registry internal to the k8s cluster. Between [30000-32767]
      --registry-pull-password string   Password for the pull-only user to access the registry
      --registry-pull-username string   Username for pull-only access to the registry
      --registry-push-password string   Password for the push-user to connect to the registry
      --registry-push-username string   Username to access to the registry Zarf is configured to use (default "zarf-push")
      --registry-secret string          Registry secret value
      --registry-url string             External registry url address to use for this Zarf cluster
      --storage-class string            Describe the StorageClass to be used
      --tmpdir string                   Specify the temporary directory to use for intermediate files
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
```

### SEE ALSO

* [zarf](zarf.md)	 - DevSecOps Airgap Toolkit


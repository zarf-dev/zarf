## zarf tools sbom completion

Generate a shell completion for Syft (listing local docker images)

### Synopsis

To load completions (docker image list):
	Bash:
	$ source <(syft completion bash)
# To load completions for each session, execute once:
	Linux:
	  $ syft completion bash > /etc/bash_completion.d/syft
	MacOS:
	  $ syft completion bash > /usr/local/etc/bash_completion.d/syft
	Zsh:
# If shell completion is not already enabled in your environment you will need
# to enable it.  You can execute the following once:
	$ echo "autoload -U compinit; compinit" >> ~/.zshrc
# To load completions for each session, execute once:
	$ syft completion zsh > "${fpath[1]}/_syft"
# You will need to start a new shell for this setup to take effect.
	Fish:
	$ syft completion fish | source
# To load completions for each session, execute once:
	$ syft completion fish > ~/.config/fish/completions/syft.fish
	

```
zarf tools sbom completion [bash|zsh|fish]
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -c, --config string         application config file
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
  -q, --quiet                 suppress all logging output
  -v, --verbose count         increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools sbom](zarf_tools_sbom.md)	 - SBOM tools provided by Anchore Syft


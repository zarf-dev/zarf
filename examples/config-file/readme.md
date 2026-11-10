This example uses a Zarf config file to configure CLI defaults, select components, and load deployment values. Run these commands from this directory:

```bash
# zarf-config.toml is loaded automatically from the current directory.
# Set ZARF_CONFIG=zarf-config.yaml to use the YAML variant instead.
zarf package create . --confirm
zarf package deploy zarf-package-config-file-*.tar.zst --confirm
```

`log_level` configures logging, `package.create.skip_sbom` controls SBOM generation, `package.deploy.components` selects the lion component, and `package.deploy.values` loads its runtime values from `deploy-values.yaml`. Command-line flags override these defaults.


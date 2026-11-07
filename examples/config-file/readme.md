<!-- // FIXME was this really nothing before?  -->
This example uses a Zarf config file to supply package template inputs and deployment values. Run these commands from this directory:

```bash
# zarf-config.toml is loaded automatically from the current directory.
# Set ZARF_CONFIG=zarf-config.yaml to use the YAML variant instead.
zarf dev template
zarf package create . --confirm
zarf package deploy zarf-package-config-file-*.tar.zst --confirm
```

`dev.template.set` supplies the inputs for `zarf.tpl.yaml`. Rendering creates `zarf.gen.yaml`, which package creation loads automatically. `package.deploy.components` selects the lion component, and `package.deploy.values` supplies its runtime values.

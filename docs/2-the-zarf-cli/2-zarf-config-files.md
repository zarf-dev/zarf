import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import FetchFileCodeBlock from '@site/src/components/FetchFileCodeBlock';

# Zarf Config Files

## Overview

Users can use a config file to easily control flags for `zarf init`, `zarf package create`, and `zarf package deploy` commands, as well as global flags (excluding `--confirm`), enabling a more straightforward and declarative workflow.

Zarf supports config files written in common configuration file formats including `toml`, `json`, `yaml`, `ini` and `props`, and by default Zarf will look for a file called `zarf-config` with one of these filenames in the current working directory.  To generate a blank config file you can run `zarf dev generate-config` with an optional output filename/format.  For example, to create an empty config file with the `my-cool-env` in the yaml format, you can use `zarf dev generate-config my-cool-env.yaml`.

To use a custom config filename, set the `ZARF_CONFIG` environment variable to the config file's path. For example, to use the `my-cool-env.yaml` config file in the current working directory, you can set the `ZARF_CONFIG` environment variable to `my-cool-env.yaml`. The `ZARF_CONFIG` environment variable can be set either in the shell or in a `.env` file in the current working directory. Note that the `ZARF_CONFIG` environment variable takes precedence over the default config file path.

Additionally, you can set any supported config parameter via an environment variable using the `ZARF_` prefix. For example, you can set the `zarf init` `--storage-class` flag by setting the `ZARF_INIT_STORAGE_CLASS` environment variable. Note that the `ZARF_` environment variable takes precedence over a config file.

While config files set default values, these values can still be overwritten by command line flags. For example, if the config file sets the log level to `info` and the command line flag is set to `debug`, the log level will be set to `debug`. The order of precedence for command line configuration is as follows:

1. Command line flags
2. Environment variables
3. Config file
4. Default values

For additional information, see the [Config File Example](../../examples/config-file/README.md).

## Config File Location

Zarf searches for the Zarf Config File from either your current working directory or the `~/.zarf/` directory if you don't specify a config file.

## Config File Examples

<Tabs queryString="init-file-examples">
<TabItem value="yaml">
<FetchFileCodeBlock src={require('../../examples/config-file/zarf-config.yaml')} fileName="zarf-config.yaml" fileFormat="yaml" />
</TabItem>
<TabItem value="toml">
<FetchFileCodeBlock src={require('../../examples/config-file/zarf-config.toml')} fileFormat="toml" fileName="zarf-config.toml" />
</TabItem>
<TabItem value="ini">
<FetchFileCodeBlock src={require('../../examples/config-file/zarf-config.ini')} fileFormat="ini"   fileName="zarf-config.ini" />
</TabItem>
<TabItem value="json">
<FetchFileCodeBlock src={require('../../examples/config-file/zarf-config.json')} fileFormat="json" fileName="zarf-config.json"  />
</TabItem>
</Tabs>

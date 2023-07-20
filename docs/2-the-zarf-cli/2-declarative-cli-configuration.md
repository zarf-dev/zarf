import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import ExampleYAML from '@site/src/components/ExampleYAML';
import FetchFileCodeBlock from '@site/src/components/FetchFileCodeBlock';

# Declarative CLI Configuration

## Overview

Users can configure the `zarf init`, `zarf package create`, and `zarf package deploy` command flags, as well as global flags (with the exception of `--confirm`), through a config file to help execute commands more declaratively.

By default, Zarf searches for a config file named `zarf-config.toml` in the current working directory. You can generate a config template for use by Zarf by executing the command `zarf prepare generate-config`, with an optional filename, in any of the supported formats, including `toml`, `json`, `yaml`, `ini` and `props`. For instance, to create a template config file with the `my-cool-env` in the yaml format, you can use the command `zarf prepare generate-config my-cool-env.yaml`.

To use a custom config file, set the `ZARF_CONFIG` environment variable to the path of the desired config file. For example, to use the `my-cool-env.yaml` config file, you can set the `ZARF_CONFIG` environment variable to `my-cool-env.yaml`. The `ZARF_CONFIG` environment variable can be set either in the shell or in the `.env` file in the current working directory. Note that the `ZARF_CONFIG` environment variable takes precedence over the default config file.

Additionally, you can also set any supported config parameter via env variable using the `ZARF_` prefix. For instance, you can set the `zarf init` `--storage-class` flag via the env variable by setting the `ZARF_INIT.STORAGE_CLASS` environment variable. Note that the `ZARF_` environment variable takes precedence over the config file.

While config files set default values, these values can still be overwritten by command line flags. For example, if the config file sets the log level to `info` and the command line flag is set to `debug`, the log level will be set to `debug`. The order of precedence for command line configuration is as follows:

1. Command line flags
2. Environment variables
3. Config file
4. Default values

For additional information, see the [Config File Example](../../examples/config-file/README.md).

## Config File Location

To use a custom config file, set the `ZARF_CONFIG` environment variable to the path of the desired config file. For example, to use the my-cool-env.yaml config file, you can set the `ZARF_CONFIG` environment variable to `my-cool-env.yaml`. The `ZARF_CONFIG` environment variable can be set either in the shell or in the .env file in the current working directory. Note that the ZARF_CONFIG environment variable takes precedence over the default config file.

It will also pickup config files from either your current working directory or `~/.zarf/` if you don't specify a config file.

## Config File Examples

<Tabs queryString="init-file-examples">
<TabItem value="yaml">
<ExampleYAML src={require('../../examples/config-file/zarf-config.yaml')} fileName="zarf-config.yaml" />
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
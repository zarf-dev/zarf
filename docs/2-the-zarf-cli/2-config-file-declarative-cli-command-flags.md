# Config File & Declarative CLI Command Flags

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

## Config File Fields

<details>
<summary>
<strong> <a name="metadata_architecture"></a>architecture</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The target cluster architecture for this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"arm64", "amd64"</code>

</blockquote>
</details>


<details open>
<summary>
<strong> <a name="init"></a>init</strong>
</summary>
&nbsp;
<blockquote>

  ## init

**Description:** Initial components and configuration to use with Zarf

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |


<details>
<summary>
<strong> <a name="init_components"></a>components</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Which optional components to install.  
E.g. --components=git-server,logging

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="init_storage_class"></a>storage_class</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The storage class to use for the registry and git server.  
E.g. --storage-class=standard

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="init_git"></a>git </strong>
</summary>
&nbsp;
<blockquote>

  ## init > git

**Description:** Any registry domains that were overridden on package create when pulling images

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                            
<details>
<summary>
<strong> <a name="init_git_pull_password"></a>pull_password</strong>
</summary>
&nbsp;

**Description:** Password for the pull-only user to access the git server

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_git_pull_username"></a>pull_username</strong>
</summary>
&nbsp;

**Description:** Username for pull-only access to the git server

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_git_push_password"></a>push_password</strong>
</summary>
&nbsp;

**Description:** Password for the push-user to access the git server

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_git_push_username"></a>push_username</strong>
</summary>
&nbsp;

**Description:** Username to access to the git server Zarf is configured to use. User must be able to create repositories via 'git push' (default "zarf-git-user")

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_git_url"></a>url</strong>
</summary>
&nbsp;

**Description:** External git server url to use for this Zarf cluster

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
</blockquote>
</details>
<details open>
<summary>
<strong> <a name="init_registry"></a>registry </strong>
</summary>
&nbsp;
<blockquote>

  ## init > registry

**Description:** Initializing with a external registry

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          

<details>
<summary>
<strong> <a name="init_registry_nodeport"></a>nodeport</strong>
</summary>
&nbsp;

**Description:** Nodeport to access a registry internal to the k8s cluster. Between [30000-32767]

|          |          |
| -------- | -------- |
| **Type** | `int` |


</details>
<details>
<summary>
<strong> <a name="init_registry_pull_password"></a>pull_password</strong>
</summary>
&nbsp;

**Description:** Password for the pull-only user to access the registry

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_registry_pull_username"></a>pull_username</strong>
</summary>
&nbsp;

**Description:** Username for pull-only access to the registry

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_registry_push_password"></a>push_password</strong>
</summary>
&nbsp;

**Description:** Password for the push-user to connect to the registry

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_registry_push_username"></a>push_username</strong>
</summary>
&nbsp;

**Description:** Username to access to the registry Zarf is configured to use (default "zarf-push")

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_registry_secret"></a>secret</strong>
</summary>
&nbsp;

**Description:** Registry secret value

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
<details>
<summary>
<strong> <a name="init_registry_url"></a>url</strong>
</summary>
&nbsp;

**Description:** External registry url address to use for this Zarf cluster

|          |          |
| -------- | -------- |
| **Type** | `string` |


</details>
</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_differentialMissing"></a>differentialMissing</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of components that were not included in this package due to differential packaging

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_5"></a>differentialMissing items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="build_OCIImportedComponents"></a>OCIImportedComponents</strong>
</summary>
&nbsp;
<blockquote>

  ## build > OCIImportedComponents

**Description:** Map of components that were imported via OCI. The keys are OCI Package URLs and values are the component names

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

<details>
<summary>
<strong> <a name="build_OCIImportedComponents_pattern1"></a>Pattern Property .*</strong>
</summary>
&nbsp;
<blockquote>

:::note
All properties whose name matches the regular expression
```.*``` ([Test](https://regex101.com/?regex=.%2A))
must respect the following conditions
:::

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>
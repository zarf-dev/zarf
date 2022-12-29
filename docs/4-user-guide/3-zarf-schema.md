# Zarf Package Schema

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackage                                                                                |

<details>
<summary><strong> <a name="kind"></a>kind *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The kind of Zarf package

|             |                       |
| ----------- | --------------------- |
| **Type**    | `enum (of string)`    |
| **Default** | `"ZarfPackageConfig"` |

:::note
Must be one of:
* "ZarfInitConfig"
* "ZarfPackageConfig"
:::

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata"></a>metadata</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Package metadata

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfMetadata                                                                               |

<details>
<summary><strong> <a name="metadata_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** Name to identify this Zarf package

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                                   |
| --------------------------------- | --------------------------------------------------------------------------------- |
| **Must match regular expression** | ```^[a-z0-9\-]+$``` [Test](https://regex101.com/?regex=%5E%5Ba-z0-9%5C-%5D%2B%24) |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_description"></a>description</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Additional information about this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_version"></a>version</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Generic string to track the package version by a package author

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_url"></a>url</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Link to package information when online

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_image"></a>image</strong>

</summary>
&nbsp;
<blockquote>

**Description:** An image URL to embed in this package for future Zarf UI listing

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_uncompressed"></a>uncompressed</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Disable compression of this package

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_architecture"></a>architecture</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The target cluster architecture of this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_yolo"></a>yolo</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="build"></a>build</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Zarf-generated package build data

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfBuildData                                                                              |

<details>
<summary><strong> <a name="build_terminal"></a>terminal *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_user"></a>user *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_architecture"></a>architecture *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_timestamp"></a>timestamp *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_version"></a>version *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components"></a>components *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** List of components to deploy in this package

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_2"></a>ZarfComponent  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponent                                                                              |

<details>
<summary><strong> <a name="components_items_name"></a>name</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The name of the component

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                                   |
| --------------------------------- | --------------------------------------------------------------------------------- |
| **Must match regular expression** | ```^[a-z0-9\-]+$``` [Test](https://regex101.com/?regex=%5E%5Ba-z0-9%5C-%5D%2B%24) |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_description"></a>description</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Message to include during package deploy describing the purpose of this component

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_default"></a>default</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Determines the default Y/N state for installing this component on package deploy

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_required"></a>required</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Do not prompt user to install this component

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_only"></a>only</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Filter when this component is included in package creation or deployment

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyTarget                                                                    |

<details>
<summary><strong> <a name="components_items_only_localOS"></a>localOS</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Only deploy component to specified OS

|          |                    |
| -------- | ------------------ |
| **Type** | `enum (of string)` |

:::note
Must be one of:
* "linux"
* "darwin"
* "windows"
:::

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_only_cluster"></a>cluster</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Only deploy component to specified clusters

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyCluster                                                                   |

<details>
<summary><strong> <a name="components_items_only_cluster_architecture"></a>architecture</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Only create and deploy to clusters of the given architecture

|          |                    |
| -------- | ------------------ |
| **Type** | `enum (of string)` |

:::note
Must be one of:
* "amd64"
* "arm64"
:::

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_only_cluster_distros"></a>distros</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Future use

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_3"></a>distros items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_group"></a>group</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Create a user selector field based on all components in the same group

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_cosignKeyPath"></a>cosignKeyPath</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Specify a path to a public key to validate signed online resources

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_import"></a>import</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Import a component from another Zarf package

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentImport                                                                        |

<details>
<summary><strong> <a name="components_items_import_name"></a>name</strong>

</summary>
&nbsp;
<blockquote>

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_import_path"></a>path *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                                                                       |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **Must match regular expression** | ```^(?!.*###ZARF_PKG_VAR_).*$``` [Test](https://regex101.com/?regex=%5E%28%3F%21.%2A%23%23%23ZARF_PKG_VAR_%29.%2A%24) |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts"></a>scripts</strong>

</summary>
&nbsp;
<blockquote>

**Description:** (Deprecated--use actions instead) Custom commands to run before or after package deployment

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/DeprecatedZarfComponentScripts                                                             |

<details>
<summary><strong> <a name="components_items_scripts_showOutput"></a>showOutput</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Show the output of the script during package deployment

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_timeoutSeconds"></a>timeoutSeconds</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Timeout in seconds for the script

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_retry"></a>retry</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Retry the script if it fails

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_prepare"></a>prepare</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Scripts to run before the component is added during package create

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_4"></a>prepare items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_before"></a>before</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Scripts to run before the component is deployed

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_5"></a>before items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_after"></a>after</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Scripts to run after the component successfully deploys

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_6"></a>after items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions"></a>actions</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Custom commands to run at various stages of a package lifecycle

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActions                                                                       |

<details>
<summary><strong> <a name="components_items_actions_create"></a>create</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run during package creation

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionSet                                                                     |

<details>
<summary><strong> <a name="components_items_actions_create_first"></a>first</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run at the start of an operation

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_7"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentAction                                                                        |

<details>
<summary><strong> <a name="components_items_actions_create_first_items_mute"></a>mute</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Hide the output of the script during package deployment

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_first_items_maxSeconds"></a>maxSeconds</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Timeout in seconds for the script

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_first_items_retry"></a>retry</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Retry the script if it fails

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_first_items_env"></a>env</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Environment variables to set for the script

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_8"></a>env items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_first_items_cmd"></a>cmd</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The script to run

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_last"></a>last</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run at the end of an operation

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_9"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_create_first_items](#components_items_actions_create_first_items)              |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_success"></a>success</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run if all operations succeed

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_10"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_create_first_items](#components_items_actions_create_first_items)              |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_create_failure"></a>failure</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run if all operations fail

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_11"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_create_first_items](#components_items_actions_create_first_items)              |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_deploy"></a>deploy</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run during package deployment

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [create](#components_items_actions_create)                                                               |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_actions_remove"></a>remove</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Actions to run during package removal

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [create](#components_items_actions_create)                                                               |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files"></a>files</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Files to place on disk during package deployment

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_12"></a>ZarfFile  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfFile                                                                                   |

<details>
<summary><strong> <a name="components_items_files_items_source"></a>source *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** Local file path or remote URL to add to the package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_shasum"></a>shasum</strong>

</summary>
&nbsp;
<blockquote>

**Description:** SHA256 checksum of the file if the source is a URL

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_target"></a>target *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The absolute or relative path where the file should be copied to during package deploy

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_executable"></a>executable</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Determines if the file should be made executable during package deploy

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_symlinks"></a>symlinks</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of symlinks to create during package deploy

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_13"></a>symlinks items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts"></a>charts</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Helm charts to install during package deploy

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_14"></a>ZarfChart  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `combining`                                                                                              |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfChart                                                                                  |

<blockquote>

| One of(Option)                                       |
| ---------------------------------------------------- |
| [url](#components_items_charts_items_oneOf_i0)       |
| [localPath](#components_items_charts_items_oneOf_i1) |

<blockquote>

## <a name="components_items_charts_items_oneOf_i0"></a>Property `url`

**Title:** url

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

### <a name="autogenerated_heading_15"></a>The following properties are required
* url

</blockquote>
<blockquote>

## <a name="components_items_charts_items_oneOf_i1"></a>Property `localPath`

**Title:** localPath

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

### <a name="autogenerated_heading_16"></a>The following properties are required
* localPath

</blockquote>

</blockquote>

<details>
<summary><strong> <a name="components_items_charts_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name of the chart to deploy; this should be the name of the chart as it is installed in the helm repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_releaseName"></a>releaseName</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The name of the release to create; defaults to the name of the chart

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_url"></a>url</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The URL of the chart repository or git url if the chart is using a git repo instead of helm repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_version"></a>version *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The version of the chart to deploy; for git-based charts this is also the tag of the git repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_namespace"></a>namespace *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The namespace to deploy the chart to

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_valuesFiles"></a>valuesFiles</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of values files to include in the package; these will be merged together

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_17"></a>valuesFiles items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_gitPath"></a>gitPath</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The path to the chart in the repo if using a git repo instead of a helm repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_localPath"></a>localPath</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The path to the chart folder

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_noWait"></a>noWait</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Wait for chart resources to be ready before continuing

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests"></a>manifests</strong>

</summary>
&nbsp;
<blockquote>

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_18"></a>ZarfManifest  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfManifest                                                                               |

<details>
<summary><strong> <a name="components_items_manifests_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** A name to give this collection of manifests; this will become the name of the dynamically-created helm chart

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_namespace"></a>namespace</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The namespace to deploy the manifests to

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_files"></a>files</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of individual K8s YAML files to deploy (in order)

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_19"></a>files items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_kustomizeAllowAnyDirectory"></a>kustomizeAllowAnyDirectory</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Allow traversing directory above the current directory if needed for kustomization

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_kustomizations"></a>kustomizations</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of kustomization paths to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_20"></a>kustomizations items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_noWait"></a>noWait</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Wait for manifest resources to be ready before continuing

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_images"></a>images</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of OCI images to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_21"></a>images items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_repos"></a>repos</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of git repos to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_22"></a>repos items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections"></a>dataInjections</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Datasets to inject into a pod in the target cluster

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_23"></a>ZarfDataInjection  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfDataInjection                                                                          |

<details>
<summary><strong> <a name="components_items_dataInjections_items_source"></a>source *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** A path to a local folder or file to inject into the given target pod + container

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target"></a>target *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The target pod + container to inject the data into

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfContainerTarget                                                                        |

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_namespace"></a>namespace *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The namespace to target for data injection

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_selector"></a>selector *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The K8s selector to target for data injection

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_container"></a>container *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The container to target for data injection

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_path"></a>path *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The path to copy the data to in the container

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_compress"></a>compress</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Compress the data before transmitting using gzip.  Note: this requires support for tar/gzip locally and in the target image.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="variables"></a>variables</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Variable template values applied on deploy for K8s resources

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_24"></a>ZarfPackageVariable  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackageVariable                                                                        |

<details>
<summary><strong> <a name="variables_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name to be used for the variable

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                         |
| --------------------------------- | ----------------------------------------------------------------------- |
| **Must match regular expression** | ```^[A-Z_]+$``` [Test](https://regex101.com/?regex=%5E%5BA-Z_%5D%2B%24) |

</blockquote>
</details>

<details>
<summary><strong> <a name="variables_items_description"></a>description</strong>

</summary>
&nbsp;
<blockquote>

**Description:** A description of the variable to be used when prompting the user a value

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="variables_items_default"></a>default</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The default value to use for the variable

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="variables_items_prompt"></a>prompt</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Whether to prompt the user for input for this variable

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="constants"></a>constants</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Constant template values applied on deploy for K8s resources

|          |         |
| -------- | ------- |
| **Type** | `array` |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_25"></a>ZarfPackageConstant  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackageConstant                                                                        |

<details>
<summary><strong> <a name="constants_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name to be used for the constant

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                         |
| --------------------------------- | ----------------------------------------------------------------------- |
| **Must match regular expression** | ```^[A-Z_]+$``` [Test](https://regex101.com/?regex=%5E%5BA-Z_%5D%2B%24) |

</blockquote>
</details>

<details>
<summary><strong> <a name="constants_items_value"></a>value *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The value to set for the constant during deploy

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary><strong> <a name="constants_items_description"></a>description</strong>

</summary>
&nbsp;
<blockquote>

**Description:** A description of the constant to explain its purpose on package create or deploy confirmation prompts

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

----------------------------------------------------------------------------------------------------------------------------
Generated from [zarf.schema.json](https://github.com/defenseunicorns/zarf/blob/master/zarf.schema.json)

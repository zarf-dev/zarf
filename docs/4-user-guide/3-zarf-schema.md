# Zarf Package Schema

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfPackage                                                                                                         |

<details>
<summary><strong> <a name="kind"></a>kind *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The kind of Zarf package

| Type        | `enum (of string)`    |
| ----------- | --------------------- |
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

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfMetadata                                                                                                        |

<details>
<summary><strong> <a name="metadata_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** Name to identify this Zarf package

| Type | `string` |
| ---- | -------- |

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

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_version"></a>version</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Generic string to track the package version by a package author

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_url"></a>url</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Link to package information when online

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_image"></a>image</strong>

</summary>
&nbsp;
<blockquote>

**Description:** An image URL to embed in this package for future Zarf UI listing

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_uncompressed"></a>uncompressed</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Disable compression of this package

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="metadata_architecture"></a>architecture</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The target cluster architecture of this package

| Type | `string` |
| ---- | -------- |

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

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfBuildData                                                                                                       |

<details>
<summary><strong> <a name="build_terminal"></a>terminal *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_user"></a>user *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_architecture"></a>architecture *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_timestamp"></a>timestamp *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="build_version"></a>version *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

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

| Type | `array` |
| ---- | ------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_2"></a>ZarfComponent  

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfComponent                                                                                                       |

<details>
<summary><strong> <a name="components_items_name"></a>name</strong>

</summary>
&nbsp;
<blockquote>

**Description:** The name of the component

| Type | `string` |
| ---- | -------- |

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

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_default"></a>default</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Determines the default Y/N state for installing this component on package deploy

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_required"></a>required</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Do not prompt user to install this component

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_only"></a>only</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Filter when this component is included in package creation or deployment

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyTarget                                                                                             |

<details>
<summary><strong> <a name="components_items_only_localOS"></a>localOS</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Only deploy component to specified OS

| Type | `enum (of string)` |
| ---- | ------------------ |

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

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyCluster                                                                                            |

<details>
<summary><strong> <a name="components_items_only_cluster_architecture"></a>architecture</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Only create and deploy to clusters of the given architecture

| Type | `enum (of string)` |
| ---- | ------------------ |

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

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_3"></a>distros items  

| Type | `string` |
| ---- | -------- |

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

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_cosignKeyPath"></a>cosignKeyPath</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Specify a path to a public key to validate signed online resources

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_import"></a>import</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Import a component from another Zarf package

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfComponentImport                                                                                                 |

<details>
<summary><strong> <a name="components_items_import_name"></a>name</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_import_path"></a>path *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_variables"></a>variables</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Dynamic template values for K8s resources

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

<details>
<summary><strong> <a name="components_items_variables_pattern1"></a>Pattern Property .*</strong>

</summary>
&nbsp;
<blockquote>

:::note
All properties whose name matches the regular expression
```.*``` ([Test](https://regex101.com/?regex=.%2A))
must respect the following conditions
:::

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts"></a>scripts</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Custom commands to run before or after package deployment

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfComponentScripts                                                                                                |

<details>
<summary><strong> <a name="components_items_scripts_showOutput"></a>showOutput</strong>

</summary>
&nbsp;
<blockquote>

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_timeoutSeconds"></a>timeoutSeconds</strong>

</summary>
&nbsp;
<blockquote>

| Type | `integer` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_retry"></a>retry</strong>

</summary>
&nbsp;
<blockquote>

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_before"></a>before</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_4"></a>before items  

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_scripts_after"></a>after</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_5"></a>after items  

| Type | `string` |
| ---- | -------- |

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

| Type | `array` |
| ---- | ------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_6"></a>ZarfFile  

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfFile                                                                                                            |

<details>
<summary><strong> <a name="components_items_files_items_source"></a>source *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_shasum"></a>shasum</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_target"></a>target *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_executable"></a>executable</strong>

</summary>
&nbsp;
<blockquote>

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_files_items_symlinks"></a>symlinks</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_7"></a>symlinks items  

| Type | `string` |
| ---- | -------- |

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

| Type | `array` |
| ---- | ------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_8"></a>ZarfChart  

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfChart                                                                                                           |

<details>
<summary><strong> <a name="components_items_charts_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_releaseName"></a>releaseName</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_url"></a>url *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_version"></a>version *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_namespace"></a>namespace *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_valuesFiles"></a>valuesFiles</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_9"></a>valuesFiles items  

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_charts_items_gitPath"></a>gitPath</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests"></a>manifests</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array` |
| ---- | ------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_10"></a>ZarfManifest  

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfManifest                                                                                                        |

<details>
<summary><strong> <a name="components_items_manifests_items_name"></a>name *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_namespace"></a>namespace</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_files"></a>files</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_11"></a>files items  

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_kustomizeAllowAnyDirectory"></a>kustomizeAllowAnyDirectory</strong>

</summary>
&nbsp;
<blockquote>

| Type | `boolean` |
| ---- | --------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_manifests_items_kustomizations"></a>kustomizations</strong>

</summary>
&nbsp;
<blockquote>

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_12"></a>kustomizations items  

| Type | `string` |
| ---- | -------- |

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

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_13"></a>images items  

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_repos"></a>repos</strong>

</summary>
&nbsp;
<blockquote>

**Description:** List of git repos to include in the package

| Type | `array of string` |
| ---- | ----------------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_14"></a>repos items  

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections"></a>dataInjections</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Datasets to inject into a pod in the target cluster

| Type | `array` |
| ---- | ------- |

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

 ## <a name="autogenerated_heading_15"></a>ZarfDataInjection  

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfDataInjection                                                                                                   |

<details>
<summary><strong> <a name="components_items_dataInjections_items_source"></a>source *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target"></a>target *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type                      | `object`                                                                                                                          |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |
| **Defined in**            | #/definitions/ZarfContainerTarget                                                                                                 |

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_namespace"></a>namespace *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_selector"></a>selector *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_container"></a>container</strong>

</summary>
&nbsp;
<blockquote>

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

<details>
<summary><strong> <a name="components_items_dataInjections_items_target_path"></a>path *</strong>

</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary><strong> <a name="seed"></a>seed</strong>

</summary>
&nbsp;
<blockquote>

**Description:** Special image only used for ZarfInitConfig packages when used with the Zarf Injector

| Type | `string` |
| ---- | -------- |

</blockquote>
</details>

----------------------------------------------------------------------------------------------------------------------------
Generated from [zarf.schema.json](https://github.com/defenseunicorns/zarf/blob/master/zarf.schema.json) on 2022-07-12 at 22:24:30 +0000
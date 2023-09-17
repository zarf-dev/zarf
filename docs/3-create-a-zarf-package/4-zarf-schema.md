# Zarf Package Schema

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackage                                                                                |

<details>
<summary>
<strong> <a name="kind"></a>kind *</strong>
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

<details open>
<summary>
<strong> <a name="metadata"></a>metadata</strong>
</summary>
&nbsp;
<blockquote>

  ## metadata

**Description:** Package metadata

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfMetadata                                                                               |

<details>
<summary>
<strong> <a name="metadata_name"></a>name *</strong>
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
<summary>
<strong> <a name="metadata_description"></a>description</strong>
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
<summary>
<strong> <a name="metadata_version"></a>version</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Generic string set by a package author to track the package version (Note: ZarfInitConfigs will always be versioned to the CLIVersion they were created with)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_url"></a>url</strong>
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
<summary>
<strong> <a name="metadata_image"></a>image</strong>
</summary>
&nbsp;
<blockquote>

**Description:** An image URL to embed in this package (Reserved for future use in Zarf UI)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_uncompressed"></a>uncompressed</strong>
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

<details>
<summary>
<strong> <a name="metadata_yolo"></a>yolo</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_authors"></a>authors</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Comma-separated list of package authors (including contact info)

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Example:**

<code>
"Doug &#60;hello@defenseunicorns.com&#62;&#44; Pepr &#60;hello@defenseunicorns.com&#62;"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_documentation"></a>documentation</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Link to package documentation when online

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_source"></a>source</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Link to package source code when online

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_vendor"></a>vendor</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Name of the distributing entity, organization or individual.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="metadata_aggregateChecksum"></a>aggregateChecksum</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Checksum of a checksums.txt file that contains checksums all the layers within the package.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="build"></a>build</strong>
</summary>
&nbsp;
<blockquote>

  ## build

**Description:** Zarf-generated package build data

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfBuildData                                                                              |

<details>
<summary>
<strong> <a name="build_terminal"></a>terminal *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The machine name that created this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_user"></a>user *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The username who created this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_architecture"></a>architecture *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The architecture this package was created on

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_timestamp"></a>timestamp *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The timestamp when this package was created

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_version"></a>version *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The version of Zarf used to build this package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_migrations"></a>migrations</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Any migrations that have been run on this package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_4"></a>migrations items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="build_differential"></a>differential</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether this package was created with differential components

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="build_registryOverrides"></a>registryOverrides</strong>
</summary>
&nbsp;
<blockquote>

  ## build > registryOverrides

**Description:** Any registry domains that were overridden on package create when pulling images

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

<details>
<summary>
<strong> <a name="build_registryOverrides_pattern1"></a>Pattern Property .*</strong>
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

<details>
<summary>
<strong> <a name="build_lastNonBreakingVersion"></a>lastNonBreakingVersion</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The minimum version of Zarf that does not have breaking package structure changes

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components"></a>components *</strong>
</summary>
&nbsp;
<blockquote>

  ## components
![Required](https://img.shields.io/badge/Required-red)

**Description:** List of components to deploy in this package

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_6"></a>ZarfComponent  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponent                                                                              |

<details>
<summary>
<strong> <a name="components_items_name"></a>name *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

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
<summary>
<strong> <a name="components_items_description"></a>description</strong>
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
<summary>
<strong> <a name="components_items_default"></a>default</strong>
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
<summary>
<strong> <a name="components_items_required"></a>required</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Do not prompt user to install this component

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_only"></a>only</strong>
</summary>
&nbsp;
<blockquote>

  ## components > only

**Description:** Filter when this component is included in package creation or deployment

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyTarget                                                                    |

<details>
<summary>
<strong> <a name="components_items_only_localOS"></a>localOS</strong>
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

<details open>
<summary>
<strong> <a name="components_items_only_cluster"></a>cluster</strong>
</summary>
&nbsp;
<blockquote>

  ## components > only > cluster

**Description:** Only deploy component to specified clusters

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentOnlyCluster                                                                   |

<details>
<summary>
<strong> <a name="components_items_only_cluster_architecture"></a>architecture</strong>
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
<summary>
<strong> <a name="components_items_only_cluster_distros"></a>distros</strong>
</summary>
&nbsp;
<blockquote>

**Description:** A list of kubernetes distros this package works with (Reserved for future use)

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_7"></a>distros items  

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
<summary>
<strong> <a name="components_items_group"></a>group</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Create a user selector field based on all components in the same group

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_import"></a>import</strong>
</summary>
&nbsp;
<blockquote>

  ## components > import

**Description:** Import a component from another Zarf package

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentImport                                                                        |

<details>
<summary>
<strong> <a name="components_items_import_name"></a>name</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The name of the component to import from the referenced zarf.yaml

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_import_path"></a>path</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The relative path to a directory containing a zarf.yaml to import from

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                                                                         |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| **Must match regular expression** | ```^(?!.*###ZARF_PKG_TMPL_).*$``` [Test](https://regex101.com/?regex=%5E%28%3F%21.%2A%23%23%23ZARF_PKG_TMPL_%29.%2A%24) |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_import_url"></a>url</strong>
</summary>
&nbsp;
<blockquote>

**Description:** [beta] The URL to a Zarf package to import via OCI

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                                                                                           |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| **Must match regular expression** | ```^oci://(?!.*###ZARF_PKG_TMPL_).*$``` [Test](https://regex101.com/?regex=%5Eoci%3A%2F%2F%28%3F%21.%2A%23%23%23ZARF_PKG_TMPL_%29.%2A%24) |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_files"></a>files</strong>
</summary>
&nbsp;
<blockquote>

  ## components > files

**Description:** Files or folders to place on disk during package deployment

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_8"></a>ZarfFile  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfFile                                                                                   |

<details>
<summary>
<strong> <a name="components_items_files_items_source"></a>source *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** Local folder or file path or remote URL to pull into the package

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_files_items_shasum"></a>shasum</strong>
</summary>
&nbsp;
<blockquote>

**Description:** (files only) Optional SHA256 checksum of the file

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_files_items_target"></a>target *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The absolute or relative path where the file or folder should be copied to during package deploy

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_files_items_executable"></a>executable</strong>
</summary>
&nbsp;
<blockquote>

**Description:** (files only) Determines if the file should be made executable during package deploy

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_files_items_symlinks"></a>symlinks</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of symlinks to create during package deploy

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_9"></a>symlinks items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_files_items_extractPath"></a>extractPath</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Local folder or file to be extracted from a 'source' archive

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_charts"></a>charts</strong>
</summary>
&nbsp;
<blockquote>

  ## components > charts

**Description:** Helm charts to install during package deploy

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_10"></a>ZarfChart  

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

### <a name="components_items_charts_items_oneOf_i0"></a>Property `url`

**Title:** url

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

#### <a name="autogenerated_heading_2"></a>The following properties are required
* url

</blockquote>
<blockquote>

### <a name="components_items_charts_items_oneOf_i1"></a>Property `localPath`

**Title:** localPath

|                           |                                                                                                                                   |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                                          |
| **Additional properties** | [![Any type: allowed](https://img.shields.io/badge/Any%20type-allowed-green)](# "Additional Properties of any type are allowed.") |

#### <a name="autogenerated_heading_2"></a>The following properties are required
* localPath

</blockquote>

</blockquote>

<details>
<summary>
<strong> <a name="components_items_charts_items_name"></a>name *</strong>
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
<summary>
<strong> <a name="components_items_charts_items_releaseName"></a>releaseName</strong>
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
<summary>
<strong> <a name="components_items_charts_items_url"></a>url</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The URL of the OCI registry, chart repository, or git repo where the helm chart is stored

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"OCI registry: oci://ghcr.io/stefanprodan/charts/podinfo", "helm chart repo: https://stefanprodan.github.io/podinfo", "git repo: https://github.com/stefanprodan/podinfo"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_charts_items_version"></a>version</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The version of the chart to deploy; for git-based charts this is also the tag of the git repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_charts_items_namespace"></a>namespace *</strong>
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
<summary>
<strong> <a name="components_items_charts_items_valuesFiles"></a>valuesFiles</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of local values file paths or remote URLs to include in the package; these will be merged together

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_11"></a>valuesFiles items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_charts_items_gitPath"></a>gitPath</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The path to the chart in the repo if using a git repo instead of a helm repo

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Example:**

<code>
"charts/your-chart"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_charts_items_localPath"></a>localPath</strong>
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
<summary>
<strong> <a name="components_items_charts_items_noWait"></a>noWait</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to not wait for chart resources to be ready before continuing

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_manifests"></a>manifests</strong>
</summary>
&nbsp;
<blockquote>

  ## components > manifests

**Description:** Kubernetes manifests to be included in a generated Helm chart on package deploy

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_12"></a>ZarfManifest  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfManifest                                                                               |

<details>
<summary>
<strong> <a name="components_items_manifests_items_name"></a>name *</strong>
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
<summary>
<strong> <a name="components_items_manifests_items_namespace"></a>namespace</strong>
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
<summary>
<strong> <a name="components_items_manifests_items_files"></a>files</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of local K8s YAML files or remote URLs to deploy (in order)

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_13"></a>files items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_manifests_items_kustomizeAllowAnyDirectory"></a>kustomizeAllowAnyDirectory</strong>
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
<summary>
<strong> <a name="components_items_manifests_items_kustomizations"></a>kustomizations</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of local kustomization paths or remote URLs to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_14"></a>kustomizations items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_manifests_items_noWait"></a>noWait</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to not wait for manifest resources to be ready before continuing

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_images"></a>images</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of OCI images to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_15"></a>images items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_repos"></a>repos</strong>
</summary>
&nbsp;
<blockquote>

**Description:** List of git repos to include in the package

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_16"></a>repos items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_dataInjections"></a>dataInjections</strong>
</summary>
&nbsp;
<blockquote>

  ## components > dataInjections

**Description:** Datasets to inject into a container in the target cluster

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_17"></a>ZarfDataInjection  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfDataInjection                                                                          |

<details>
<summary>
<strong> <a name="components_items_dataInjections_items_source"></a>source *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** Either a path to a local folder/file or a remote URL of a file to inject into the given target pod + container

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_dataInjections_items_target"></a>target *</strong>
</summary>
&nbsp;
<blockquote>

  ## components > dataInjections > target
![Required](https://img.shields.io/badge/Required-red)

**Description:** The target pod + container to inject the data into

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfContainerTarget                                                                        |

<details>
<summary>
<strong> <a name="components_items_dataInjections_items_target_namespace"></a>namespace *</strong>
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
<summary>
<strong> <a name="components_items_dataInjections_items_target_selector"></a>selector *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The K8s selector to target for data injection

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Example:**

<code>
"app&#61;data-injection"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_dataInjections_items_target_container"></a>container *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The container name to target for data injection

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_dataInjections_items_target_path"></a>path *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The path within the container to copy the data into

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_dataInjections_items_compress"></a>compress</strong>
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

<details open>
<summary>
<strong> <a name="components_items_extensions"></a>extensions</strong>
</summary>
&nbsp;
<blockquote>

  ## components > extensions

**Description:** Extend component functionality with additional features

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentExtensions                                                                    |

<details open>
<summary>
<strong> <a name="components_items_extensions_bigbang"></a>bigbang</strong>
</summary>
&nbsp;
<blockquote>

  ## components > extensions > bigbang

**Description:** Configurations for installing Big Bang and Flux in the cluster

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/BigBang                                                                                    |

<details>
<summary>
<strong> <a name="components_items_extensions_bigbang_version"></a>version *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The version of Big Bang to use

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_extensions_bigbang_repo"></a>repo</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Override repo to pull Big Bang from instead of Repo One

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_extensions_bigbang_valuesFiles"></a>valuesFiles</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The list of values files to pass to Big Bang; these will be merged together

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_18"></a>valuesFiles items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_extensions_bigbang_skipFlux"></a>skipFlux</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to skip deploying flux; Defaults to false

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_extensions_bigbang_fluxPatchFiles"></a>fluxPatchFiles</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Optional paths to Flux kustomize strategic merge patch files

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_19"></a>fluxPatchFiles items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions"></a>actions</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions

**Description:** Custom commands to run at various stages of a package lifecycle

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActions                                                                       |

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate"></a>onCreate</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate

**Description:** Actions to run during package creation

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionSet                                                                     |

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults"></a>defaults</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > defaults

**Description:** Default configuration for all actions in this set

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionDefaults                                                                |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_mute"></a>mute</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Hide the output of commands during execution (default false)

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_maxTotalSeconds"></a>maxTotalSeconds</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Default timeout in seconds for commands (default to 0

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_maxRetries"></a>maxRetries</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Retry commands given number of times if they fail (default 0)

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_dir"></a>dir</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Working directory for commands (default CWD)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_env"></a>env</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Additional environment variables for commands

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_20"></a>env items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_shell"></a>shell</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > defaults > shell

**Description:** (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionShell                                                                   |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_shell_windows"></a>windows</strong>
</summary>
&nbsp;
<blockquote>

**Description:** (default 'powershell') Indicates a preference for the shell to use on Windows systems (note that choosing 'cmd' will turn off migrations like touch -> New-Item)

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"powershell", "cmd", "pwsh", "sh", "bash", "gsh"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_shell_linux"></a>linux</strong>
</summary>
&nbsp;
<blockquote>

**Description:** (default 'sh') Indicates a preference for the shell to use on Linux systems

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"sh", "bash", "fish", "zsh", "pwsh"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_defaults_shell_darwin"></a>darwin</strong>
</summary>
&nbsp;
<blockquote>

**Description:** (default 'sh') Indicates a preference for the shell to use on macOS systems

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"sh", "bash", "fish", "zsh", "pwsh"</code>

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before"></a>before</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before

**Description:** Actions to run at the start of an operation

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_21"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentAction                                                                        |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_mute"></a>mute</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Hide the output of the command during package deployment (default false)

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_maxTotalSeconds"></a>maxTotalSeconds</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Timeout in seconds for the command (default to 0

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_maxRetries"></a>maxRetries</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Retry the command if it fails up to given number of times (default 0)

|          |           |
| -------- | --------- |
| **Type** | `integer` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_dir"></a>dir</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The working directory to run the command in (default is CWD)

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_env"></a>env</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Additional environment variables to set for the command

|          |                   |
| -------- | ----------------- |
| **Type** | `array of string` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_22"></a>env items  

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_cmd"></a>cmd</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The command to run. Must specify either cmd or wait for the action to do anything.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_shell"></a>shell</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before > shell

**Description:** (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [shell](#components_items_actions_onCreate_defaults_shell)                                               |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables"></a>setVariables</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before > setVariables

**Description:** (onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package.

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_23"></a>ZarfComponentActionSetVariable  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionSetVariable                                                             |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables_items_name"></a>name *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name to be used for the variable

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                               |
| --------------------------------- | ----------------------------------------------------------------------------- |
| **Must match regular expression** | ```^[A-Z0-9_]+$``` [Test](https://regex101.com/?regex=%5E%5BA-Z0-9_%5D%2B%24) |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables_items_sensitive"></a>sensitive</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to mark this variable as sensitive to not print it in the Zarf log

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables_items_autoIndent"></a>autoIndent</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_VAR_.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables_items_pattern"></a>pattern</strong>
</summary>
&nbsp;
<blockquote>

**Description:** An optional regex pattern that a variable value must match before a package deployment can continue.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_setVariables_items_type"></a>type</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Changes the handling of a variable to load contents differently (i.e. from a file rather than as a raw variable - templated files should be kept below 1 MiB)

|          |                    |
| -------- | ------------------ |
| **Type** | `enum (of string)` |

:::note
Must be one of:
* "raw"
* "file"
:::

</blockquote>
</details>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_description"></a>description</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Description of the action to be displayed during package execution instead of the command

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait"></a>wait</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before > wait

**Description:** Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info.

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionWait                                                                    |

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_cluster"></a>cluster</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before > wait > cluster

**Description:** Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified.

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionWaitCluster                                                             |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_cluster_kind"></a>kind *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The kind of resource to wait for

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"Pod", "Deployment)"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_cluster_name"></a>name *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name of the resource or selector to wait for

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"podinfo", "app&#61;podinfo"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_cluster_namespace"></a>namespace</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The namespace of the resource to wait for

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_cluster_condition"></a>condition</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The condition or jsonpath state to wait for; defaults to exist

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"Ready", "Available"</code>

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_network"></a>network</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > before > wait > network

**Description:** Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified.

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfComponentActionWaitNetwork                                                             |

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_network_protocol"></a>protocol *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The protocol to wait for

|          |                    |
| -------- | ------------------ |
| **Type** | `enum (of string)` |

:::note
Must be one of:
* "tcp"
* "http"
* "https"
:::

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_network_address"></a>address *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The address to wait for

|          |          |
| -------- | -------- |
| **Type** | `string` |

**Examples:**

<code>
"localhost:8080", "1.1.1.1"</code>

</blockquote>
</details>

<details>
<summary>
<strong> <a name="components_items_actions_onCreate_before_items_wait_network_code"></a>code</strong>
</summary>
&nbsp;
<blockquote>

**Description:** The HTTP status code to wait for if using http or https

|          |           |
| -------- | --------- |
| **Type** | `integer` |

**Examples:**

<code>
200, 404</code>

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_after"></a>after</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > after

**Description:** Actions to run at the end of an operation

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_24"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_onCreate_before_items](#components_items_actions_onCreate_before_items)        |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_onSuccess"></a>onSuccess</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > onSuccess

**Description:** Actions to run if all operations succeed

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_25"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_onCreate_before_items](#components_items_actions_onCreate_before_items)        |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onCreate_onFailure"></a>onFailure</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onCreate > onFailure

**Description:** Actions to run if all operations fail

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_26"></a>ZarfComponentAction  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [components_items_actions_onCreate_before_items](#components_items_actions_onCreate_before_items)        |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onDeploy"></a>onDeploy</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onDeploy

**Description:** Actions to run during package deployment

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [onCreate](#components_items_actions_onCreate)                                                           |

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="components_items_actions_onRemove"></a>onRemove</strong>
</summary>
&nbsp;
<blockquote>

  ## components > actions > onRemove

**Description:** Actions to run during package removal

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Same definition as**    | [onCreate](#components_items_actions_onCreate)                                                           |

</blockquote>
</details>

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="constants"></a>constants</strong>
</summary>
&nbsp;
<blockquote>

  ## constants

**Description:** Constant template values applied on deploy for K8s resources

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_27"></a>ZarfPackageConstant  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackageConstant                                                                        |

<details>
<summary>
<strong> <a name="constants_items_name"></a>name *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name to be used for the constant

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                               |
| --------------------------------- | ----------------------------------------------------------------------------- |
| **Must match regular expression** | ```^[A-Z0-9_]+$``` [Test](https://regex101.com/?regex=%5E%5BA-Z0-9_%5D%2B%24) |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="constants_items_value"></a>value *</strong>
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
<summary>
<strong> <a name="constants_items_description"></a>description</strong>
</summary>
&nbsp;
<blockquote>

**Description:** A description of the constant to explain its purpose on package create or deploy confirmation prompts

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="constants_items_autoIndent"></a>autoIndent</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_CONST_.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="constants_items_pattern"></a>pattern</strong>
</summary>
&nbsp;
<blockquote>

**Description:** An optional regex pattern that a constant value must match before a package can be created.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

</blockquote>
</details>

<details open>
<summary>
<strong> <a name="variables"></a>variables</strong>
</summary>
&nbsp;
<blockquote>

  ## variables

**Description:** Variable template values applied on deploy for K8s resources

|          |         |
| -------- | ------- |
| **Type** | `array` |

![Min Items: N/A](https://img.shields.io/badge/Min%20Items%3A%20N/A-gold)
![Max Items: N/A](https://img.shields.io/badge/Max%20Items%3A%20N/A-gold)
![Item unicity: False](https://img.shields.io/badge/Item%20unicity%3A%20False-gold)
![Additional items: N/A](https://img.shields.io/badge/Additional%20items%3A%20N/A-gold)

 ### <a name="autogenerated_heading_28"></a>ZarfPackageVariable  

|                           |                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                                                 |
| **Additional properties** | [![Not allowed](https://img.shields.io/badge/Not%20allowed-red)](# "Additional Properties not allowed.") |
| **Defined in**            | #/definitions/ZarfPackageVariable                                                                        |

<details>
<summary>
<strong> <a name="variables_items_name"></a>name *</strong>
</summary>
&nbsp;
<blockquote>

![Required](https://img.shields.io/badge/Required-red)

**Description:** The name to be used for the variable

|          |          |
| -------- | -------- |
| **Type** | `string` |

| Restrictions                      |                                                                               |
| --------------------------------- | ----------------------------------------------------------------------------- |
| **Must match regular expression** | ```^[A-Z0-9_]+$``` [Test](https://regex101.com/?regex=%5E%5BA-Z0-9_%5D%2B%24) |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="variables_items_description"></a>description</strong>
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
<summary>
<strong> <a name="variables_items_default"></a>default</strong>
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
<summary>
<strong> <a name="variables_items_prompt"></a>prompt</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to prompt the user for input for this variable

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="variables_items_sensitive"></a>sensitive</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to mark this variable as sensitive to not print it in the Zarf log

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="variables_items_autoIndent"></a>autoIndent</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_VAR_.

|          |           |
| -------- | --------- |
| **Type** | `boolean` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="variables_items_pattern"></a>pattern</strong>
</summary>
&nbsp;
<blockquote>

**Description:** An optional regex pattern that a variable value must match before a package can be deployed.

|          |          |
| -------- | -------- |
| **Type** | `string` |

</blockquote>
</details>

<details>
<summary>
<strong> <a name="variables_items_type"></a>type</strong>
</summary>
&nbsp;
<blockquote>

**Description:** Changes the handling of a variable to load contents differently (i.e. from a file rather than as a raw variable - templated files should be kept below 1 MiB)

|          |                    |
| -------- | ------------------ |
| **Type** | `enum (of string)` |

:::note
Must be one of:
* "raw"
* "file"
:::

</blockquote>
</details>

</blockquote>
</details>

----------------------------------------------------------------------------------------------------------------------------
Generated from [zarf.schema.json](https://github.com/defenseunicorns/zarf/blob/main/zarf.schema.json)

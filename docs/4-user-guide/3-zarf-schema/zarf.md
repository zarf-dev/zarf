# Zarf Package Schema

```txt
undefined
```



| Abstract               | Extensible | Status         | Identifiable            | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                 |
| :--------------------- | :--------- | :------------- | :---------------------- | :---------------- | :-------------------- | :------------------ | :------------------------------------------------------------------------- |
| Cannot be instantiated | Yes        | Unknown status | Unknown identifiability | Forbidden         | Allowed               | none                | [zarf.schema.json](../../../build/zarf.schema.json "open original schema") |

## Zarf Package Type

unknown ([Zarf Package](zarf.md))

# Zarf Package Definitions

## Definitions group ZarfBuildData

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfBuildData"}
```

| Property                      | Type     | Required | Nullable       | Defined by                                                                                                                               |
| :---------------------------- | :------- | :------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| [terminal](#terminal)         | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata-properties-terminal.md "undefined#/definitions/ZarfBuildData/properties/terminal")         |
| [user](#user)                 | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata-properties-user.md "undefined#/definitions/ZarfBuildData/properties/user")                 |
| [architecture](#architecture) | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata-properties-architecture.md "undefined#/definitions/ZarfBuildData/properties/architecture") |
| [timestamp](#timestamp)       | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata-properties-timestamp.md "undefined#/definitions/ZarfBuildData/properties/timestamp")       |
| [string](#string)             | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata-properties-string.md "undefined#/definitions/ZarfBuildData/properties/string")             |

### terminal



`terminal`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata-properties-terminal.md "undefined#/definitions/ZarfBuildData/properties/terminal")

#### terminal Type

`string`

### user



`user`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata-properties-user.md "undefined#/definitions/ZarfBuildData/properties/user")

#### user Type

`string`

### architecture



`architecture`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata-properties-architecture.md "undefined#/definitions/ZarfBuildData/properties/architecture")

#### architecture Type

`string`

### timestamp



`timestamp`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata-properties-timestamp.md "undefined#/definitions/ZarfBuildData/properties/timestamp")

#### timestamp Type

`string`

### string



`string`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata-properties-string.md "undefined#/definitions/ZarfBuildData/properties/string")

#### string Type

`string`

## Definitions group ZarfChart

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfChart"}
```

| Property                    | Type     | Required | Nullable       | Defined by                                                                                                                     |
| :-------------------------- | :------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------- |
| [name](#name)               | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-name.md "undefined#/definitions/ZarfChart/properties/name")               |
| [releaseName](#releasename) | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-releasename.md "undefined#/definitions/ZarfChart/properties/releaseName") |
| [url](#url)                 | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-url.md "undefined#/definitions/ZarfChart/properties/url")                 |
| [version](#version)         | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-version.md "undefined#/definitions/ZarfChart/properties/version")         |
| [namespace](#namespace)     | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-namespace.md "undefined#/definitions/ZarfChart/properties/namespace")     |
| [valuesFiles](#valuesfiles) | `array`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-valuesfiles.md "undefined#/definitions/ZarfChart/properties/valuesFiles") |
| [gitPath](#gitpath)         | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfchart-properties-gitpath.md "undefined#/definitions/ZarfChart/properties/gitPath")         |

### name



`name`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-name.md "undefined#/definitions/ZarfChart/properties/name")

#### name Type

`string`

### releaseName



`releaseName`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-releasename.md "undefined#/definitions/ZarfChart/properties/releaseName")

#### releaseName Type

`string`

### url



`url`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-url.md "undefined#/definitions/ZarfChart/properties/url")

#### url Type

`string`

### version



`version`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-version.md "undefined#/definitions/ZarfChart/properties/version")

#### version Type

`string`

### namespace



`namespace`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-namespace.md "undefined#/definitions/ZarfChart/properties/namespace")

#### namespace Type

`string`

### valuesFiles



`valuesFiles`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-valuesfiles.md "undefined#/definitions/ZarfChart/properties/valuesFiles")

#### valuesFiles Type

`string[]`

### gitPath



`gitPath`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfchart-properties-gitpath.md "undefined#/definitions/ZarfChart/properties/gitPath")

#### gitPath Type

`string`

## Definitions group ZarfComponent

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfComponent"}
```

| Property                          | Type      | Required | Nullable       | Defined by                                                                                                                                   |
| :-------------------------------- | :-------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name-1)                   | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-name.md "undefined#/definitions/ZarfComponent/properties/name")                     |
| [description](#description)       | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-description.md "undefined#/definitions/ZarfComponent/properties/description")       |
| [default](#default)               | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-default.md "undefined#/definitions/ZarfComponent/properties/default")               |
| [required](#required)             | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-required.md "undefined#/definitions/ZarfComponent/properties/required")             |
| [files](#files)                   | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-files.md "undefined#/definitions/ZarfComponent/properties/files")                   |
| [charts](#charts)                 | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-charts.md "undefined#/definitions/ZarfComponent/properties/charts")                 |
| [manifests](#manifests)           | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-manifests.md "undefined#/definitions/ZarfComponent/properties/manifests")           |
| [images](#images)                 | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-images.md "undefined#/definitions/ZarfComponent/properties/images")                 |
| [repos](#repos)                   | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-repos.md "undefined#/definitions/ZarfComponent/properties/repos")                   |
| [dataInjections](#datainjections) | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-datainjections.md "undefined#/definitions/ZarfComponent/properties/dataInjections") |
| [scripts](#scripts)               | `object`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts.md "undefined#/definitions/ZarfComponent/properties/scripts")                           |
| [import](#import)                 | `object`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentimport.md "undefined#/definitions/ZarfComponent/properties/import")                             |
| [cosignKeyPath](#cosignkeypath)   | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-cosignkeypath.md "undefined#/definitions/ZarfComponent/properties/cosignKeyPath")   |
| [variables](#variables)           | `object`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-variables.md "undefined#/definitions/ZarfComponent/properties/variables")           |
| [group](#group)                   | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-group.md "undefined#/definitions/ZarfComponent/properties/group")                   |

### name



`name`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-name.md "undefined#/definitions/ZarfComponent/properties/name")

#### name Type

`string`

### description



`description`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-description.md "undefined#/definitions/ZarfComponent/properties/description")

#### description Type

`string`

### default



`default`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-default.md "undefined#/definitions/ZarfComponent/properties/default")

#### default Type

`boolean`

### required



`required`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-required.md "undefined#/definitions/ZarfComponent/properties/required")

#### required Type

`boolean`

### files



`files`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarffile.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-files.md "undefined#/definitions/ZarfComponent/properties/files")

#### files Type

`object[]` ([Details](zarf-definitions-zarffile.md))

### charts



`charts`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfchart.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-charts.md "undefined#/definitions/ZarfComponent/properties/charts")

#### charts Type

`object[]` ([Details](zarf-definitions-zarfchart.md))

### manifests



`manifests`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfmanifest.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-manifests.md "undefined#/definitions/ZarfComponent/properties/manifests")

#### manifests Type

`object[]` ([Details](zarf-definitions-zarfmanifest.md))

### images



`images`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-images.md "undefined#/definitions/ZarfComponent/properties/images")

#### images Type

`string[]`

### repos



`repos`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-repos.md "undefined#/definitions/ZarfComponent/properties/repos")

#### repos Type

`string[]`

### dataInjections



`dataInjections`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfdatainjection.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-datainjections.md "undefined#/definitions/ZarfComponent/properties/dataInjections")

#### dataInjections Type

`object[]` ([Details](zarf-definitions-zarfdatainjection.md))

### scripts



`scripts`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponentscripts.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts.md "undefined#/definitions/ZarfComponent/properties/scripts")

#### scripts Type

`object` ([Details](zarf-definitions-zarfcomponentscripts.md))

### import



`import`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponentimport.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentimport.md "undefined#/definitions/ZarfComponent/properties/import")

#### import Type

`object` ([Details](zarf-definitions-zarfcomponentimport.md))

### cosignKeyPath



`cosignKeyPath`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-cosignkeypath.md "undefined#/definitions/ZarfComponent/properties/cosignKeyPath")

#### cosignKeyPath Type

`string`

### variables



`variables`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponent-properties-variables.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-variables.md "undefined#/definitions/ZarfComponent/properties/variables")

#### variables Type

`object` ([Details](zarf-definitions-zarfcomponent-properties-variables.md))

### group



`group`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-group.md "undefined#/definitions/ZarfComponent/properties/group")

#### group Type

`string`

## Definitions group ZarfComponentImport

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfComponentImport"}
```

| Property        | Type     | Required | Nullable       | Defined by                                                                                                                           |
| :-------------- | :------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name-2) | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentimport-properties-name.md "undefined#/definitions/ZarfComponentImport/properties/name") |
| [path](#path)   | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentimport-properties-path.md "undefined#/definitions/ZarfComponentImport/properties/path") |

### name



`name`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentimport-properties-name.md "undefined#/definitions/ZarfComponentImport/properties/name")

#### name Type

`string`

### path



`path`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentimport-properties-path.md "undefined#/definitions/ZarfComponentImport/properties/path")

#### path Type

`string`

## Definitions group ZarfComponentScripts

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfComponentScripts"}
```

| Property                          | Type      | Required | Nullable       | Defined by                                                                                                                                                 |
| :-------------------------------- | :-------- | :------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [showOutput](#showoutput)         | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-showoutput.md "undefined#/definitions/ZarfComponentScripts/properties/showOutput")         |
| [timeoutSeconds](#timeoutseconds) | `integer` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-timeoutseconds.md "undefined#/definitions/ZarfComponentScripts/properties/timeoutSeconds") |
| [retry](#retry)                   | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-retry.md "undefined#/definitions/ZarfComponentScripts/properties/retry")                   |
| [before](#before)                 | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-before.md "undefined#/definitions/ZarfComponentScripts/properties/before")                 |
| [after](#after)                   | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-after.md "undefined#/definitions/ZarfComponentScripts/properties/after")                   |

### showOutput



`showOutput`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-showoutput.md "undefined#/definitions/ZarfComponentScripts/properties/showOutput")

#### showOutput Type

`boolean`

### timeoutSeconds



`timeoutSeconds`

*   is optional

*   Type: `integer`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-timeoutseconds.md "undefined#/definitions/ZarfComponentScripts/properties/timeoutSeconds")

#### timeoutSeconds Type

`integer`

### retry



`retry`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-retry.md "undefined#/definitions/ZarfComponentScripts/properties/retry")

#### retry Type

`boolean`

### before



`before`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-before.md "undefined#/definitions/ZarfComponentScripts/properties/before")

#### before Type

`string[]`

### after



`after`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts-properties-after.md "undefined#/definitions/ZarfComponentScripts/properties/after")

#### after Type

`string[]`

## Definitions group ZarfContainerTarget

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfContainerTarget"}
```

| Property                  | Type     | Required | Nullable       | Defined by                                                                                                                                     |
| :------------------------ | :------- | :------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- |
| [namespace](#namespace-1) | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-namespace.md "undefined#/definitions/ZarfContainerTarget/properties/namespace") |
| [selector](#selector)     | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-selector.md "undefined#/definitions/ZarfContainerTarget/properties/selector")   |
| [container](#container)   | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-container.md "undefined#/definitions/ZarfContainerTarget/properties/container") |
| [path](#path-1)           | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-path.md "undefined#/definitions/ZarfContainerTarget/properties/path")           |

### namespace



`namespace`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-namespace.md "undefined#/definitions/ZarfContainerTarget/properties/namespace")

#### namespace Type

`string`

### selector



`selector`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-selector.md "undefined#/definitions/ZarfContainerTarget/properties/selector")

#### selector Type

`string`

### container



`container`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-container.md "undefined#/definitions/ZarfContainerTarget/properties/container")

#### container Type

`string`

### path



`path`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-path.md "undefined#/definitions/ZarfContainerTarget/properties/path")

#### path Type

`string`

## Definitions group ZarfDataInjection

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfDataInjection"}
```

| Property          | Type     | Required | Nullable       | Defined by                                                                                                                           |
| :---------------- | :------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------------- |
| [source](#source) | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfdatainjection-properties-source.md "undefined#/definitions/ZarfDataInjection/properties/source") |
| [target](#target) | `object` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget.md "undefined#/definitions/ZarfDataInjection/properties/target")                 |

### source



`source`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfdatainjection-properties-source.md "undefined#/definitions/ZarfDataInjection/properties/source")

#### source Type

`string`

### target



`target`

*   is required

*   Type: `object` ([Details](zarf-definitions-zarfcontainertarget.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget.md "undefined#/definitions/ZarfDataInjection/properties/target")

#### target Type

`object` ([Details](zarf-definitions-zarfcontainertarget.md))

## Definitions group ZarfFile

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfFile"}
```

| Property                  | Type      | Required | Nullable       | Defined by                                                                                                                 |
| :------------------------ | :-------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------- |
| [source](#source-1)       | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-source.md "undefined#/definitions/ZarfFile/properties/source")         |
| [shasum](#shasum)         | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-shasum.md "undefined#/definitions/ZarfFile/properties/shasum")         |
| [target](#target-1)       | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-target.md "undefined#/definitions/ZarfFile/properties/target")         |
| [executable](#executable) | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-executable.md "undefined#/definitions/ZarfFile/properties/executable") |
| [symlinks](#symlinks)     | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-symlinks.md "undefined#/definitions/ZarfFile/properties/symlinks")     |

### source



`source`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-source.md "undefined#/definitions/ZarfFile/properties/source")

#### source Type

`string`

### shasum



`shasum`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-shasum.md "undefined#/definitions/ZarfFile/properties/shasum")

#### shasum Type

`string`

### target



`target`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-target.md "undefined#/definitions/ZarfFile/properties/target")

#### target Type

`string`

### executable



`executable`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-executable.md "undefined#/definitions/ZarfFile/properties/executable")

#### executable Type

`boolean`

### symlinks



`symlinks`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-symlinks.md "undefined#/definitions/ZarfFile/properties/symlinks")

#### symlinks Type

`string[]`

## Definitions group ZarfManifest

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfManifest"}
```

| Property                                                  | Type      | Required | Nullable       | Defined by                                                                                                                                                         |
| :-------------------------------------------------------- | :-------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name-3)                                           | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-name.md "undefined#/definitions/ZarfManifest/properties/name")                                             |
| [namespace](#namespace-2)                                 | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-namespace.md "undefined#/definitions/ZarfManifest/properties/namespace")                                   |
| [files](#files-1)                                         | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-files.md "undefined#/definitions/ZarfManifest/properties/files")                                           |
| [kustomizeAllowAnyDirectory](#kustomizeallowanydirectory) | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizeallowanydirectory.md "undefined#/definitions/ZarfManifest/properties/kustomizeAllowAnyDirectory") |
| [kustomizations](#kustomizations)                         | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizations.md "undefined#/definitions/ZarfManifest/properties/kustomizations")                         |

### name



`name`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-name.md "undefined#/definitions/ZarfManifest/properties/name")

#### name Type

`string`

### namespace



`namespace`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-namespace.md "undefined#/definitions/ZarfManifest/properties/namespace")

#### namespace Type

`string`

### files



`files`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-files.md "undefined#/definitions/ZarfManifest/properties/files")

#### files Type

`string[]`

### kustomizeAllowAnyDirectory



`kustomizeAllowAnyDirectory`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizeallowanydirectory.md "undefined#/definitions/ZarfManifest/properties/kustomizeAllowAnyDirectory")

#### kustomizeAllowAnyDirectory Type

`boolean`

### kustomizations



`kustomizations`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizations.md "undefined#/definitions/ZarfManifest/properties/kustomizations")

#### kustomizations Type

`string[]`

## Definitions group ZarfMetadata

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfMetadata"}
```

| Property                        | Type      | Required | Nullable       | Defined by                                                                                                                             |
| :------------------------------ | :-------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name-4)                 | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-name.md "undefined#/definitions/ZarfMetadata/properties/name")                 |
| [description](#description-1)   | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-description.md "undefined#/definitions/ZarfMetadata/properties/description")   |
| [version](#version-1)           | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-version.md "undefined#/definitions/ZarfMetadata/properties/version")           |
| [url](#url-1)                   | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-url.md "undefined#/definitions/ZarfMetadata/properties/url")                   |
| [image](#image)                 | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-image.md "undefined#/definitions/ZarfMetadata/properties/image")               |
| [uncompressed](#uncompressed)   | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-uncompressed.md "undefined#/definitions/ZarfMetadata/properties/uncompressed") |
| [architecture](#architecture-1) | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata-properties-architecture.md "undefined#/definitions/ZarfMetadata/properties/architecture") |

### name



`name`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-name.md "undefined#/definitions/ZarfMetadata/properties/name")

#### name Type

`string`

### description



`description`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-description.md "undefined#/definitions/ZarfMetadata/properties/description")

#### description Type

`string`

### version



`version`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-version.md "undefined#/definitions/ZarfMetadata/properties/version")

#### version Type

`string`

### url



`url`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-url.md "undefined#/definitions/ZarfMetadata/properties/url")

#### url Type

`string`

### image



`image`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-image.md "undefined#/definitions/ZarfMetadata/properties/image")

#### image Type

`string`

### uncompressed



`uncompressed`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-uncompressed.md "undefined#/definitions/ZarfMetadata/properties/uncompressed")

#### uncompressed Type

`boolean`

### architecture



`architecture`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata-properties-architecture.md "undefined#/definitions/ZarfMetadata/properties/architecture")

#### architecture Type

`string`

## Definitions group ZarfPackage

Reference this group by using

```json
{"$ref":"undefined#/definitions/ZarfPackage"}
```

| Property                  | Type     | Required | Nullable       | Defined by                                                                                                                       |
| :------------------------ | :------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------------- |
| [kind](#kind)             | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-kind.md "undefined#/definitions/ZarfPackage/properties/kind")             |
| [metadata](#metadata)     | `object` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata.md "undefined#/definitions/ZarfPackage/properties/metadata")                        |
| [build](#build)           | `object` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata.md "undefined#/definitions/ZarfPackage/properties/build")                          |
| [components](#components) | `array`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-components.md "undefined#/definitions/ZarfPackage/properties/components") |
| [seed](#seed)             | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-seed.md "undefined#/definitions/ZarfPackage/properties/seed")             |

### kind



`kind`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-kind.md "undefined#/definitions/ZarfPackage/properties/kind")

#### kind Type

`string`

### metadata



`metadata`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfmetadata.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata.md "undefined#/definitions/ZarfPackage/properties/metadata")

#### metadata Type

`object` ([Details](zarf-definitions-zarfmetadata.md))

### build



`build`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfbuilddata.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata.md "undefined#/definitions/ZarfPackage/properties/build")

#### build Type

`object` ([Details](zarf-definitions-zarfbuilddata.md))

### components



`components`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfcomponent.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-components.md "undefined#/definitions/ZarfPackage/properties/components")

#### components Type

`object[]` ([Details](zarf-definitions-zarfcomponent.md))

### seed



`seed`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-seed.md "undefined#/definitions/ZarfPackage/properties/seed")

#### seed Type

`string`

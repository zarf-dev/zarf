# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfPackage/properties/components/items
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## items Type

`object` ([Details](zarf-definitions-zarfcomponent.md))

# items Properties

| Property                          | Type      | Required | Nullable       | Defined by                                                                                                                                   |
| :-------------------------------- | :-------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name)                     | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcomponent-properties-name.md "undefined#/definitions/ZarfComponent/properties/name")                     |
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

## name



`name`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-name.md "undefined#/definitions/ZarfComponent/properties/name")

### name Type

`string`

## description



`description`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-description.md "undefined#/definitions/ZarfComponent/properties/description")

### description Type

`string`

## default



`default`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-default.md "undefined#/definitions/ZarfComponent/properties/default")

### default Type

`boolean`

## required



`required`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-required.md "undefined#/definitions/ZarfComponent/properties/required")

### required Type

`boolean`

## files



`files`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarffile.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-files.md "undefined#/definitions/ZarfComponent/properties/files")

### files Type

`object[]` ([Details](zarf-definitions-zarffile.md))

## charts



`charts`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfchart.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-charts.md "undefined#/definitions/ZarfComponent/properties/charts")

### charts Type

`object[]` ([Details](zarf-definitions-zarfchart.md))

## manifests



`manifests`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfmanifest.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-manifests.md "undefined#/definitions/ZarfComponent/properties/manifests")

### manifests Type

`object[]` ([Details](zarf-definitions-zarfmanifest.md))

## images



`images`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-images.md "undefined#/definitions/ZarfComponent/properties/images")

### images Type

`string[]`

## repos



`repos`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-repos.md "undefined#/definitions/ZarfComponent/properties/repos")

### repos Type

`string[]`

## dataInjections



`dataInjections`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfdatainjection.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-datainjections.md "undefined#/definitions/ZarfComponent/properties/dataInjections")

### dataInjections Type

`object[]` ([Details](zarf-definitions-zarfdatainjection.md))

## scripts



`scripts`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponentscripts.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentscripts.md "undefined#/definitions/ZarfComponent/properties/scripts")

### scripts Type

`object` ([Details](zarf-definitions-zarfcomponentscripts.md))

## import



`import`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponentimport.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponentimport.md "undefined#/definitions/ZarfComponent/properties/import")

### import Type

`object` ([Details](zarf-definitions-zarfcomponentimport.md))

## cosignKeyPath



`cosignKeyPath`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-cosignkeypath.md "undefined#/definitions/ZarfComponent/properties/cosignKeyPath")

### cosignKeyPath Type

`string`

## variables



`variables`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfcomponent-properties-variables.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-variables.md "undefined#/definitions/ZarfComponent/properties/variables")

### variables Type

`object` ([Details](zarf-definitions-zarfcomponent-properties-variables.md))

## group



`group`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcomponent-properties-group.md "undefined#/definitions/ZarfComponent/properties/group")

### group Type

`string`

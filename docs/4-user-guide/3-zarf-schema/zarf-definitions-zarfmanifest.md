# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfComponent/properties/manifests/items
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## items Type

`object` ([Details](zarf-definitions-zarfmanifest.md))

# items Properties

| Property                                                  | Type      | Required | Nullable       | Defined by                                                                                                                                                         |
| :-------------------------------------------------------- | :-------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [name](#name)                                             | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-name.md "undefined#/definitions/ZarfManifest/properties/name")                                             |
| [namespace](#namespace)                                   | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-namespace.md "undefined#/definitions/ZarfManifest/properties/namespace")                                   |
| [files](#files)                                           | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-files.md "undefined#/definitions/ZarfManifest/properties/files")                                           |
| [kustomizeAllowAnyDirectory](#kustomizeallowanydirectory) | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizeallowanydirectory.md "undefined#/definitions/ZarfManifest/properties/kustomizeAllowAnyDirectory") |
| [kustomizations](#kustomizations)                         | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizations.md "undefined#/definitions/ZarfManifest/properties/kustomizations")                         |

## name



`name`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-name.md "undefined#/definitions/ZarfManifest/properties/name")

### name Type

`string`

## namespace



`namespace`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-namespace.md "undefined#/definitions/ZarfManifest/properties/namespace")

### namespace Type

`string`

## files



`files`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-files.md "undefined#/definitions/ZarfManifest/properties/files")

### files Type

`string[]`

## kustomizeAllowAnyDirectory



`kustomizeAllowAnyDirectory`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizeallowanydirectory.md "undefined#/definitions/ZarfManifest/properties/kustomizeAllowAnyDirectory")

### kustomizeAllowAnyDirectory Type

`boolean`

## kustomizations



`kustomizations`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmanifest-properties-kustomizations.md "undefined#/definitions/ZarfManifest/properties/kustomizations")

### kustomizations Type

`string[]`

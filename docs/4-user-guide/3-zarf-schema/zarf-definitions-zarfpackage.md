# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfPackage
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## ZarfPackage Type

`object` ([Details](zarf-definitions-zarfpackage.md))

# ZarfPackage Properties

| Property                  | Type     | Required | Nullable       | Defined by                                                                                                                       |
| :------------------------ | :------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------------- |
| [kind](#kind)             | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-kind.md "undefined#/definitions/ZarfPackage/properties/kind")             |
| [metadata](#metadata)     | `object` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfmetadata.md "undefined#/definitions/ZarfPackage/properties/metadata")                        |
| [build](#build)           | `object` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfbuilddata.md "undefined#/definitions/ZarfPackage/properties/build")                          |
| [components](#components) | `array`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-components.md "undefined#/definitions/ZarfPackage/properties/components") |
| [seed](#seed)             | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfpackage-properties-seed.md "undefined#/definitions/ZarfPackage/properties/seed")             |

## kind



`kind`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-kind.md "undefined#/definitions/ZarfPackage/properties/kind")

### kind Type

`string`

## metadata



`metadata`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfmetadata.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfmetadata.md "undefined#/definitions/ZarfPackage/properties/metadata")

### metadata Type

`object` ([Details](zarf-definitions-zarfmetadata.md))

## build



`build`

*   is optional

*   Type: `object` ([Details](zarf-definitions-zarfbuilddata.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfbuilddata.md "undefined#/definitions/ZarfPackage/properties/build")

### build Type

`object` ([Details](zarf-definitions-zarfbuilddata.md))

## components



`components`

*   is optional

*   Type: `object[]` ([Details](zarf-definitions-zarfcomponent.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-components.md "undefined#/definitions/ZarfPackage/properties/components")

### components Type

`object[]` ([Details](zarf-definitions-zarfcomponent.md))

## seed



`seed`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfpackage-properties-seed.md "undefined#/definitions/ZarfPackage/properties/seed")

### seed Type

`string`

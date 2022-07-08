# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfDataInjection/properties/target
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## target Type

`object` ([Details](zarf-definitions-zarfcontainertarget.md))

# target Properties

| Property                | Type     | Required | Nullable       | Defined by                                                                                                                                     |
| :---------------------- | :------- | :------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- |
| [namespace](#namespace) | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-namespace.md "undefined#/definitions/ZarfContainerTarget/properties/namespace") |
| [selector](#selector)   | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-selector.md "undefined#/definitions/ZarfContainerTarget/properties/selector")   |
| [container](#container) | `string` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-container.md "undefined#/definitions/ZarfContainerTarget/properties/container") |
| [path](#path)           | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget-properties-path.md "undefined#/definitions/ZarfContainerTarget/properties/path")           |

## namespace



`namespace`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-namespace.md "undefined#/definitions/ZarfContainerTarget/properties/namespace")

### namespace Type

`string`

## selector



`selector`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-selector.md "undefined#/definitions/ZarfContainerTarget/properties/selector")

### selector Type

`string`

## container



`container`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-container.md "undefined#/definitions/ZarfContainerTarget/properties/container")

### container Type

`string`

## path



`path`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget-properties-path.md "undefined#/definitions/ZarfContainerTarget/properties/path")

### path Type

`string`

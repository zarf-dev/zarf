# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfComponent/properties/dataInjections/items
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## items Type

`object` ([Details](zarf-definitions-zarfdatainjection.md))

# items Properties

| Property          | Type     | Required | Nullable       | Defined by                                                                                                                           |
| :---------------- | :------- | :------- | :------------- | :----------------------------------------------------------------------------------------------------------------------------------- |
| [source](#source) | `string` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfdatainjection-properties-source.md "undefined#/definitions/ZarfDataInjection/properties/source") |
| [target](#target) | `object` | Required | cannot be null | [Zarf Package](zarf-definitions-zarfcontainertarget.md "undefined#/definitions/ZarfDataInjection/properties/target")                 |

## source



`source`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfdatainjection-properties-source.md "undefined#/definitions/ZarfDataInjection/properties/source")

### source Type

`string`

## target



`target`

*   is required

*   Type: `object` ([Details](zarf-definitions-zarfcontainertarget.md))

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarfcontainertarget.md "undefined#/definitions/ZarfDataInjection/properties/target")

### target Type

`object` ([Details](zarf-definitions-zarfcontainertarget.md))

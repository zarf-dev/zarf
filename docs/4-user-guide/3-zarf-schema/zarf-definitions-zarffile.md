# Untitled object in Zarf Package Schema

```txt
undefined#/definitions/ZarfComponent/properties/files/items
```



| Abstract            | Extensible | Status         | Identifiable | Custom Properties | Additional Properties | Access Restrictions | Defined In                                                                   |
| :------------------ | :--------- | :------------- | :----------- | :---------------- | :-------------------- | :------------------ | :--------------------------------------------------------------------------- |
| Can be instantiated | No         | Unknown status | No           | Forbidden         | Forbidden             | none                | [zarf.schema.json\*](../../../build/zarf.schema.json "open original schema") |

## items Type

`object` ([Details](zarf-definitions-zarffile.md))

# items Properties

| Property                  | Type      | Required | Nullable       | Defined by                                                                                                                 |
| :------------------------ | :-------- | :------- | :------------- | :------------------------------------------------------------------------------------------------------------------------- |
| [source](#source)         | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-source.md "undefined#/definitions/ZarfFile/properties/source")         |
| [shasum](#shasum)         | `string`  | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-shasum.md "undefined#/definitions/ZarfFile/properties/shasum")         |
| [target](#target)         | `string`  | Required | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-target.md "undefined#/definitions/ZarfFile/properties/target")         |
| [executable](#executable) | `boolean` | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-executable.md "undefined#/definitions/ZarfFile/properties/executable") |
| [symlinks](#symlinks)     | `array`   | Optional | cannot be null | [Zarf Package](zarf-definitions-zarffile-properties-symlinks.md "undefined#/definitions/ZarfFile/properties/symlinks")     |

## source



`source`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-source.md "undefined#/definitions/ZarfFile/properties/source")

### source Type

`string`

## shasum



`shasum`

*   is optional

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-shasum.md "undefined#/definitions/ZarfFile/properties/shasum")

### shasum Type

`string`

## target



`target`

*   is required

*   Type: `string`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-target.md "undefined#/definitions/ZarfFile/properties/target")

### target Type

`string`

## executable



`executable`

*   is optional

*   Type: `boolean`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-executable.md "undefined#/definitions/ZarfFile/properties/executable")

### executable Type

`boolean`

## symlinks



`symlinks`

*   is optional

*   Type: `string[]`

*   cannot be null

*   defined in: [Zarf Package](zarf-definitions-zarffile-properties-symlinks.md "undefined#/definitions/ZarfFile/properties/symlinks")

### symlinks Type

`string[]`

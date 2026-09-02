# Go `struct` Converter

A library for converting between Go structs.

```go
chain := converter.NewFuncChain(V1toV2, V2toV3)

chain.Convert(myV1struct, &myV3struct)
```

## Details

At its core, this library provides a `Convert` function, which automatically
handles converting fields with the same name, and "convertable"
types. Some examples are:
* `string` -> `string`
* `string` -> `*string`
* `int` -> `string`
* `string` -> `[]string`

The automatic conversions are implemented when there is an obvious way
to convert between the types. A lot more automatic conversions happen
-- see [the converter tests](converter_test.go) for a more comprehensive
list of what is currently supported.

Not everything can be handled automatically, however, so there is also
a way to specify custom conversions. By implementing a function that
adheres to some basic forms. Any custom conversions found during object graph
traversal will attempt automatic  conversion, and pass the resulting structs
to the custom conversion function.

Additionally, and maybe most importantly, there is a `converter.FuncChain` available,
which orchestrates conversions across _multiple versions_ of structs. This could
be thought of similar to database migrations: given a starting struct and a target
struct, the `FuncChain.Convert` function iterates through every intermediary migration
in order to arrive at the target struct.

## Basic Usage

To illustrate usage we'll start with a few basic structs:

```go
type V1 struct {
  Name     string
  OldField string
}

type V2 struct {
  Name     string
  NewField string // this was a renamed field
}

type V3 struct {
  Name       []string
  FinalField []string // this field was renamed and the type was changed
}
```

Given these type definitions, we can easily set up a conversion chain
like this:

```go
func V1toV2(from V1, to *V2) error { // forward migration
    to.NewField = from.OldField
    return nil
}

func V2toV3(from V2, to *V3) error { // forward migration
    to.FinalField = []string{from.NewField}
    return nil
}

...

chain := converter.NewFuncChain(V1toV2, V2toV3)
```

This chain can then be used to convert from an _older version_ to a _newer 
version_. This is because we only supplied _forward_ migrations.

This chain can be used to convert from a `V1` struct to a `V3` struct easily,
like this:

```go
v1 := // somehow get a populated v1 struct
v3 := V3{}
chain.Convert(v1, &v3)
```

Since we've defined our chain as `V1` &rarr; `V2` &rarr; `V3`, the chain will execute
conversions to all intermediary structs (`V2`, in this case) and ultimately end
when we've populated the `v3` instance.

Note we haven't needed to define any conversions on the `Name` field of any structs
since this one is convertible between structs: `string` &rarr; `string` &rarr; `[]string`.

## Backwards Migrations

If we wanted to _also_ provide backwards migrations, we could also easily add functions for
this.

```go
func V2toV1(from V2, to *V1) error { // backward migration
    to.OldField = from.NewField
    return nil
}

func V3toV2(from V3, to *V2) error { // backward migration
    to.NewField = from.FinalField[0]
    return nil
}

chain := converter.NewFuncChain(V1toV2, V2toV1, V2toV3, V3toV2)
```

At this point we could convert in either direction, for example a 
`V3` struct could convert to a `V1` struct, with the caveat that there
may be data loss, due to whatever changes were made to the data shapes.

## Contributing

If you would like to contribute to this repository, please see the
[CONTRIBUTING.md](CONTRIBUTING.md).

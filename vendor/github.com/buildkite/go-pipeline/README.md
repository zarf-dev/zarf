# go-pipeline

[![Build status](https://badge.buildkite.com/1fad7fb9610283e4955ea4ec4c88faca52162b637fea61821e.svg)](https://buildkite.com/buildkite/go-pipeline)
[![Go Reference](https://pkg.go.dev/badge/github.com/buildkite/go-pipeline.svg)](https://pkg.go.dev/github.com/buildkite/go-pipeline)

`go-pipeline` is a Go library used for building and modifying Buildkite pipelines in golang. It's used internally by the [Buildkite Agent](https://github.com/buildkite/agent) to inspect and sign pipelines prior to uploading them, but is also useful for building tools that generate pipelines.

## Installation

To install, run

```
go get -u github.com/buildkite/go-pipeline
```

This will add go-pipeline to your go.mod file, and make it available for use in your project.

## Usage

### Loading a pipeline from yaml

```go
const aPipeline = `
env:
  MOUNTAIN: cotopaxi
  COUNTRY: ecuador

steps:
  - command: echo "hello world"
  - wait
  - command: echo "goodbye world"
`

p, err := pipeline.Parse(strings.NewReader(aPipeline))
if err != nil {
  panic(err)
}

pretty.Println(p)
// &pipeline.Pipeline{
//   Env: &ordered.Map[string,string]{
//     items: {
//       {Key:"MOUNTAIN", Value:"cotopaxi", deleted:false},
//       {Key:"COUNTRY", Value:"ecuador", deleted:false},
//     },
//     index: {"MOUNTAIN":0, "COUNTRY":1},
//   },
//   Steps: {
//     &pipeline.CommandStep{
//       Command:         "echo \"hello world\"",
//       Env:             {},
//       RemainingFields: {},
//     },
//     &pipeline.WaitStep{
//       Scalar:  "wait",
//       Contents: {},
//     },
//     &pipeline.CommandStep{
//       Command:         "echo \"goodbye world\"",
//       Env:             {},
//       RemainingFields: {},
//     },
//   },
//   RemainingFields: {},
// }
```

### Marshalling to YAML or JSON
```go
aPipeline := `...`
p, err := pipeline.Parse(strings.NewReader(aPipeline))
if err != nil {
  // ...
}

//... modify the pipeline

// Marshal to YAML
b, err := yaml.Marshal(p)
if err != nil {
  // ...
}

// Marshal to JSON
b, err := json.Marshal(p)
if err != nil {
  // ...
}
```

## Caveats
The pipeline object model (`Pipeline`, `Steps`, `Plugin`, etc) have these caveats:
- It is incomplete: there may be fields accepted by the API that are not listed. Do not treat Pipeline, CommandStep, etc, as comprehensive reference guides for how to write a pipeline.
- It normalises: unmarshaling accepts a variety of step forms, but marshaling back out produces more normalised output. An unmarshal/marshal round-trip may produce different output.
- It is non-canonical: using the object model does not guarantee that a pipeline will be accepted by the pipeline upload API.

Notably, most of the structs defined by this module only contain the elements of a pipeline (and steps) necessary for the agent to understand, and are (at the time of writing) not comprehensive. Where relevant - that is, where there are more fields that are not included in the struct - the `RemainingFields` field is used to capture the remaining fields as a `map[string]any`. This allows pipelines to be loaded and modified without losing information, even if the pipeline contains fields that are not yet understood by the agent.

For example, the command step:
```YAML
command: echo "hello world"
env:
  FOO: bar
  BAZ: qux
artifact_paths:
  - "logs/**/*"
  - "coverage/**/*"
parallelism: 5
```

would be represented in go as:
```go
&pipeline.CommandStep{
  Command: `echo "hello world"`,
  Env: ordered.MapFromItems(
    ordered.TupleSS("FOO", "bar"),
    ordered.TupleSS("BAZ", "qux"),
  ),
  RemainingFields: map[string]any{
    "artifact_paths": []string{"logs/**/*", "coverage/**/*"},
    "parallelism": 5,
  },
}
```

This go struct would be marshaled back out to YAML equivalent to the original input.

## What's up with the ordered module?

While implementing the pipeline module, we ran into a problem: in some cases, in the buildkite pipeline.yaml, the order of map fields is significant. Because of this, whenever the pipeline gets unmarshaled from YAML or JSON, it needs to be stored in a way that preserves the order of the fields. The `ordered` module is a simple implementation of an ordered map. In most cases, when the pipeline is dealing with user-input maps, it will store them internally as `ordered.Map`s. When the pipeline is marshaled back out to YAML or JSON, the `ordered.Map`s will be marshaled in the correct order.

## Contributing

Contributions, bugfixes, issues and PRs are always welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for more details.

// Package pipeline implements the pieces necessary for the agent to work with
// pipelines (typically in YAML or JSON form).
//
// The pipeline object model (Pipeline, Steps, Plugin, etc) have these caveats:
//   - It is incomplete: there may be fields accepted by the API that are not
//     listed. Do not treat Pipeline, CommandStep, etc, as comprehensive
//     reference guides for how to write a pipeline.
//   - It normalises: unmarshaling accepts a variety of step forms, but
//     marshaling back out produces more normalised output. An unmarshal/marshal
//     round-trip may produce different output.
//   - It is non-canonical: using the object model does not guarantee that a
//     pipeline will be accepted by the pipeline upload API.
package pipeline

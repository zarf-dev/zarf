/*
Validator APIs for SLSA Provenance v1 protos.
*/
package v1

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// all of the following errors apply to SLSA Build L1 and above
var (
	ErrBuilderRequired         = errors.New("runDetails.builder required")
	ErrBuilderIdRequired       = errors.New("runDetails.builder.id required")
	ErrBuildDefinitionRequired = errors.New("buildDefinition required")
	ErrBuildTypeRequired       = errors.New("buildDefinition.buildType required")
	ErrExternalParamsRequired  = errors.New("buildDefinition.externalParameters required")
	ErrRunDetailsRequired      = errors.New("runDetails required")
)

func (m *BuildMetadata) Validate() error {
	// check valid timestamps
	s := m.GetStartedOn()
	if s != nil {
		if err := s.CheckValid(); err != nil {
			return fmt.Errorf("buildMetadata.startedOn error: %w", err)
		}
	}

	f := m.GetFinishedOn()
	if f != nil {
		if err := f.CheckValid(); err != nil {
			return fmt.Errorf("buildMetadata.finishedOn error: %w", err)
		}
	}

	return nil
}

func (b *Builder) Validate() error {
	// the id field is required for SLSA Build L1
	if b.GetId() == "" {
		return ErrBuilderIdRequired
	}

	// check that all builderDependencies are valid RDs
	builderDeps := b.GetBuilderDependencies()
	for i, rd := range builderDeps {
		if err := rd.Validate(); err != nil {
			return fmt.Errorf("Invalid Builder.BuilderDependencies[%d]: %w", i, err)
		}
	}

	return nil
}

func (b *BuildDefinition) Validate() error {
	// the buildType field is required for SLSA Build L1
	if b.GetBuildType() == "" {
		return ErrBuildTypeRequired
	}

	// the externalParameters field is required for SLSA Build L1
	ext := b.GetExternalParameters()
	if ext == nil || proto.Equal(ext, &structpb.Struct{}) {
		return ErrExternalParamsRequired
	}

	// check that all resolvedDependencies are valid RDs
	resolvedDeps := b.GetResolvedDependencies()
	for i, rd := range resolvedDeps {
		if err := rd.Validate(); err != nil {
			return fmt.Errorf("Invalid BuildDefinition.ResolvedDependencies[%d]: %w", i, err)
		}
	}

	return nil
}

func (r *RunDetails) Validate() error {
	// the builder field is required for SLSA Build L1
	builder := r.GetBuilder()
	if builder == nil || proto.Equal(builder, &Builder{}) {
		return ErrBuilderRequired
	}

	// check the Builder
	if err := builder.Validate(); err != nil {
		return fmt.Errorf("runDetails.builder error: %w", err)
	}

	// check the Metadata, if present
	metadata := r.GetMetadata()
	if metadata != nil && !proto.Equal(metadata, &BuildMetadata{}) {
		if err := metadata.Validate(); err != nil {
			return fmt.Errorf("Invalid RunDetails.Metadata: %w", err)
		}
	}

	// check that all byproducts are valid RDs
	byproducts := r.GetByproducts()
	for i, rd := range byproducts {
		if err := rd.Validate(); err != nil {
			return fmt.Errorf("Invalid RunDetails.Byproducts[%d]: %w", i, err)
		}
	}

	return nil
}

func (p *Provenance) Validate() error {
	// the buildDefinition field is required for SLSA Build L1
	buildDef := p.GetBuildDefinition()
	if buildDef == nil || proto.Equal(buildDef, &BuildDefinition{}) {
		return ErrBuildDefinitionRequired
	}

	// check the BuildDefinition
	if err := buildDef.Validate(); err != nil {
		return fmt.Errorf("provenance.buildDefinition error: %w", err)
	}

	// the runDetails field is required for SLSA Build L1
	runDetails := p.GetRunDetails()
	if runDetails == nil || proto.Equal(runDetails, &RunDetails{}) {
		return ErrRunDetailsRequired
	}

	// check the RunDetails
	if err := runDetails.Validate(); err != nil {
		return fmt.Errorf("provenance.runDetails error: %w", err)
	}

	return nil
}

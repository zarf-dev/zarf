// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package componentartifact validates the OCI contract used for published
// v1beta1 component configurations.
package componentartifact

import (
	"fmt"
	"path"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// Artifact is an already-resolved published component and its selected OCI
// manifest. Validation deliberately performs no I/O.
type Artifact struct {
	Config   v1beta1.ComponentConfig
	Manifest ocispec.Manifest
	Platform *ocispec.Platform
}

type resourceClaim struct {
	path      string
	directory bool
	field     string
}

// Validate verifies that an artifact could have been emitted by component.Publish.
// FIXME: this function is too focused on making sure the component is 100% published by Zarf, this is not the intent. It should be focused that the user can add anything malicious from outside of Zarf
func Validate(artifact Artifact) error {
	config := artifact.Config
	if config.APIVersion != v1beta1.APIVersion {
		return fmt.Errorf("apiVersion must be %q", v1beta1.APIVersion)
	}
	if config.Kind != v1beta1.ZarfComponentConfig {
		return fmt.Errorf("kind must be %q", v1beta1.ZarfComponentConfig)
	}
	if config.Metadata.Name == "" || config.Metadata.Version == "" {
		return fmt.Errorf("metadata name and version are required")
	}
	if config.PublishData.ZarfVersion == "" {
		return fmt.Errorf("publishData.zarfVersion is required")
	}
	if len(config.Component.Import.Local) != 0 || len(config.Component.Import.Remote) != 0 {
		return fmt.Errorf("component.import must be empty in a published component")
	}
	if hasActions(config.Component.Actions.OnCreate) {
		return fmt.Errorf("component.actions.onCreate must be empty in a published component")
	}
	if errs := internalv1beta1.ValidateComponentConfig(config); len(errs) != 0 {
		return fmt.Errorf("component semantic validation failed: %w", errs)
	}
	if !variantMatchesPlatform(config.Variant, artifact.Platform) {
		return fmt.Errorf("component variant architecture does not match its OCI platform")
	}

	claims, err := resourceClaims(config)
	if err != nil {
		return err
	}
	return validateManifest(config, artifact.Manifest, claims)
}

func validateManifest(config v1beta1.ComponentConfig, manifest ocispec.Manifest, claims []resourceClaim) error {
	if manifest.SchemaVersion != 2 || manifest.MediaType != ocispec.MediaTypeImageManifest {
		return fmt.Errorf("manifest is not an OCI image manifest")
	}
	if manifest.ArtifactType != "" || manifest.Subject != nil {
		return fmt.Errorf("manifest has an unexpected artifact type or subject")
	}
	if manifest.Config.MediaType != layout.ZarfComponentConfigMediaType {
		return fmt.Errorf("config descriptor has unexpected media type %q", manifest.Config.MediaType)
	}
	if err := validDescriptor(manifest.Config); err != nil {
		return fmt.Errorf("invalid config descriptor: %w", err)
	}
	for key, want := range map[string]string{
		ocispec.AnnotationTitle:       config.Metadata.Name,
		ocispec.AnnotationDescription: config.Metadata.Description,
		ocispec.AnnotationVersion:     config.Metadata.Version,
	} {
		if manifest.Annotations[key] != want {
			return fmt.Errorf("manifest annotation %q does not match component metadata", key)
		}
	}

	mounts := make([]string, 0, len(manifest.Layers))
	hasEmpty := false
	for _, layer := range manifest.Layers {
		if layer.MediaType == ocispec.MediaTypeEmptyJSON {
			if hasEmpty || len(manifest.Layers) != 1 || layer.Digest != ocispec.DescriptorEmptyJSON.Digest || layer.Size != ocispec.DescriptorEmptyJSON.Size || len(layer.Annotations) != 0 {
				return fmt.Errorf("OCI empty layer is not the no-resource representation")
			}
			hasEmpty = true
			continue
		}
		if layer.MediaType != layout.ZarfComponentLayerMediaType {
			return fmt.Errorf("resource layer has unexpected media type %q", layer.MediaType)
		}
		if err := validDescriptor(layer); err != nil {
			return fmt.Errorf("invalid resource descriptor: %w", err)
		}
		if len(layer.Annotations) != 2 {
			return fmt.Errorf("resource layer must contain only title and mount annotations")
		}
		mount := layer.Annotations[layout.ComponentResourceMountPathAnnotation]
		if layer.Annotations[ocispec.AnnotationTitle] != mount || !validArtifactPath(mount, false) {
			return fmt.Errorf("resource layer has an invalid mount path %q", mount)
		}
		for _, previous := range mounts {
			if mount == previous || strings.HasPrefix(mount, previous+"/") || strings.HasPrefix(previous, mount+"/") {
				return fmt.Errorf("resource layer mount paths collide: %q and %q", previous, mount)
			}
		}
		mounts = append(mounts, mount)
	}
	if hasEmpty && len(claims) != 0 {
		return fmt.Errorf("OCI empty layer cannot accompany component resources")
	}
	for _, claim := range claims {
		matched := false
		for _, mount := range mounts {
			if mount == claim.path || (claim.directory && strings.HasPrefix(mount, claim.path+"/")) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("resource %s at %q has no artifact layer", claim.field, claim.path)
		}
	}
	for _, mount := range mounts {
		matched := false
		for _, claim := range claims {
			if mount == claim.path || (claim.directory && strings.HasPrefix(mount, claim.path+"/")) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("resource layer at %q is not referenced by the component config", mount)
		}
	}
	return nil
}

func validDescriptor(descriptor ocispec.Descriptor) error {
	if descriptor.Size < 0 || descriptor.Digest.Algorithm() != digest.SHA256 || descriptor.Digest.Validate() != nil {
		return fmt.Errorf("digest and size must use a non-negative sha256 descriptor")
	}
	return nil
}

func resourceClaims(config v1beta1.ComponentConfig) ([]resourceClaim, error) {
	var claims []resourceClaim
	add := func(value, field string, directory bool, localOnly bool) error {
		if value == "" {
			return nil
		}
		if helpers.IsURL(value) {
			if localOnly {
				return fmt.Errorf("%s must be an artifact resource, not a URL", field)
			}
			return nil
		}
		if !validArtifactPath(value, value == string(layout.ImagesDir)) {
			return fmt.Errorf("%s has invalid artifact path %q", field, value)
		}
		claims = append(claims, resourceClaim{path: value, directory: directory, field: field})
		return nil
	}
	for _, value := range config.Values.Files {
		if err := add(value, "values.files", false, true); err != nil {
			return nil, err
		}
	}
	if err := add(config.Values.Schema, "values.schema", false, true); err != nil {
		return nil, err
	}
	for _, chart := range config.Component.Charts {
		if chart.Local != nil {
			if err := add(chart.Local.Path, "component.charts.local.path", true, true); err != nil {
				return nil, err
			}
		}
		for _, value := range chart.ValuesFiles {
			if err := add(value.Path, "component.charts.valuesFiles.path", false, false); err != nil {
				return nil, err
			}
		}
	}
	for _, manifest := range config.Component.Manifests {
		for _, value := range manifest.Files {
			if err := add(value, "component.manifests.files", true, false); err != nil {
				return nil, err
			}
		}
		for _, value := range manifest.Kustomize.Files {
			if err := add(value, "component.manifests.kustomize.files", true, false); err != nil {
				return nil, err
			}
		}
	}
	for _, file := range config.Component.Files {
		if err := add(file.Source, "component.files.source", true, false); err != nil {
			return nil, err
		}
	}
	for _, archive := range config.Component.ImageArchives {
		if err := add(archive.Path, "component.imageArchives.path", true, true); err != nil {
			return nil, err
		}
	}
	return claims, nil
}

func validArtifactPath(value string, allowImagesRoot bool) bool {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value ||
		strings.HasPrefix(value, "../") || strings.Contains(value, "/../") || strings.HasPrefix(value, "//") ||
		(len(value) >= 2 && value[1] == ':') {
		return false
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	if strings.HasPrefix(value, "resources/") {
		return len(parts) >= 3 && parts[1] != ""
	}
	return (allowImagesRoot && value == string(layout.ImagesDir)) || strings.HasPrefix(value, string(layout.ImagesDir)+"/")
}

func hasActions(actions v1beta1.ComponentActionSet) bool {
	return actions.Defaults != nil || len(actions.Before) != 0 || len(actions.OnSuccess) != 0 || len(actions.OnFailure) != 0
}

func variantMatchesPlatform(variant v1beta1.ComponentVariant, platform *ocispec.Platform) bool {
	return (platform == nil && variant.Architecture == "") || (platform != nil && variant.Architecture == platform.Architecture)
}

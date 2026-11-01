// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom generates package SBOMs with syft. It is kept out of pkg/packager so programs that only
// load or deploy packages do not link syft.
package sbom

import (
	"context"
	"errors"
	"fmt"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/artifact"
	syftFile "github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/linux"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/directorysource"
	"github.com/anchore/syft/syft/source/filesource"
	"github.com/anchore/syft/syft/source/stereoscopesource"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// SyftGenerator creates syft-json SBOMs.
type SyftGenerator struct{}

// NewSyftGenerator returns an SBOM generator for assemble.AssembleOptions and packager.CreateOptions.
func NewSyftGenerator() *SyftGenerator {
	return &SyftGenerator{}
}

// ImageSBOM catalogs img, caching image layers in cacheDir.
func (SyftGenerator) ImageSBOM(ctx context.Context, img v1.Image, identifier, cacheDir string) ([]byte, error) {
	syftImage := image.New(img, file.NewTempDirGenerator("zarf"), cacheDir, image.WithTags(identifier))
	err := syftImage.Read()
	if err != nil {
		return nil, err
	}
	cfg := getDefaultSyftConfig()
	syftSrc := stereoscopesource.New(syftImage, stereoscopesource.ImageConfig{
		Reference: identifier,
	})
	sbom, err := syft.CreateSBOM(ctx, syftSrc, cfg)
	if err != nil {
		return nil, err
	}
	return format.Encode(*sbom, syftjson.NewFormatEncoder())
}

// FilesSBOM catalogs each of files into one SBOM with rootDir as its source.
func (SyftGenerator) FilesSBOM(ctx context.Context, rootDir string, files []string) ([]byte, error) {
	l := logger.From(ctx)
	parentSource, err := directorysource.NewFromPath(rootDir)
	if err != nil {
		return nil, err
	}
	catalog := pkg.NewCollection()
	relationships := []artifact.Relationship{}
	for index, sbomFile := range files {
		l.Info("creating file SBOMs", "file", sbomFile, "count", fmt.Sprintf("%d/%d", index+1, len(files)))
		fileSrc, err := filesource.NewFromPath(sbomFile)
		if err != nil {
			return nil, err
		}

		cfg := getDefaultSyftConfig()
		sbom, err := syft.CreateSBOM(ctx, fileSrc, cfg)
		if err != nil {
			return nil, err
		}

		for pkg := range sbom.Artifacts.Packages.Enumerate() {
			containsSource := false
			fileMetadata, ok := fileSrc.Describe().Metadata.(source.FileMetadata)
			if !ok {
				return nil, errors.New("failed to get file metadata from SBOM source")
			}

			// See if the source locations for this package contain the file Zarf indexed
			for _, location := range pkg.Locations.ToSlice() {
				if location.RealPath == fileMetadata.Path {
					containsSource = true
				}
			}

			// If the locations do not contain the source file (i.e. the package was inside a tarball), add the file source
			if !containsSource {
				sourceLocation := syftFile.NewLocation(fileMetadata.Path)
				pkg.Locations.Add(sourceLocation)
			}

			catalog.Add(pkg)
		}

		for _, r := range sbom.Relationships {
			relationships = append(relationships, artifact.Relationship{
				From: parentSource,
				To:   r.To,
				Type: r.Type,
				Data: r.Data,
			})
		}
	}
	artifact := sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name:    "zarf",
			Version: config.CLIVersion,
		},
		Source: parentSource.Describe(),
		Artifacts: sbom.Artifacts{
			Packages:          catalog,
			LinuxDistribution: &linux.Release{},
		},
		Relationships: relationships,
	}
	return format.Encode(artifact, syftjson.NewFormatEncoder())
}

func getDefaultSyftConfig() *syft.CreateSBOMConfig {
	cfg := syft.DefaultCreateSBOMConfig()
	cfg.ToolName = "zarf"
	cfg.ToolVersion = config.CLIVersion
	return cfg
}

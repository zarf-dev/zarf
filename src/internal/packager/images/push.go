// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	orasRetry "oras.land/oras-go/v2/registry/remote/retry"

	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/spdxjson"
	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

const defaultRetries = 3

// Push pushes images to a registry.
func Push(ctx context.Context, cfg PushConfig) error {
	if cfg.Retries < 1 {
		cfg.Retries = defaultRetries
	}
	if cfg.ResponseHeaderTimeout <= 0 {
		cfg.ResponseHeaderTimeout = 10 * time.Second
	}
	cfg.ImageList = helpers.Unique(cfg.ImageList)
	toPush := map[string]struct{}{}
	for _, img := range cfg.ImageList {
		toPush[img.Reference] = struct{}{}
	}
	l := logger.From(ctx)
	registryURL := cfg.RegistryInfo.Address
	err := addRefNameAnnotationToImages(cfg.SourceDirectory)
	if err != nil {
		return err
	}

	src, err := oci.NewWithContext(ctx, cfg.SourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to instantiate oci directory: %w", err)
	}

	err = retry.Do(func() error {
		// reset concurrency to user-provided value on each component retry
		ociConcurrency := cfg.OCIConcurrency

		// Include tunnel connection in case the port forward breaks, for example, a registry pod could spin down / restart
		var tunnel *cluster.Tunnel
		if cfg.Cluster != nil {
			var err error
			registryURL, tunnel, err = cfg.Cluster.ConnectToZarfRegistryEndpoint(ctx, cfg.RegistryInfo)
			if err != nil {
				return err
			}
			if tunnel != nil {
				defer tunnel.Close()
			}
		}

		client := &auth.Client{
			Client: orasRetry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(registryURL, auth.Credential{
				Username: cfg.RegistryInfo.PushUsername,
				Password: cfg.RegistryInfo.PushPassword,
			}),
		}

		client.Client.Transport, err = orasTransport(cfg.InsecureSkipTLSVerify, cfg.ResponseHeaderTimeout)
		if err != nil {
			return err
		}

		plainHTTP := cfg.PlainHTTP

		if dns.IsLocalhost(registryURL) && !cfg.PlainHTTP {
			var err error
			plainHTTP, err = shouldUsePlainHTTP(ctx, registryURL, client)
			if err != nil {
				return err
			}
		}

		pushImage := func(srcName, dstName string) error {
			remoteRepo := &orasRemote.Repository{
				PlainHTTP: plainHTTP,
				Client:    client,
			}
			remoteRepo.Reference, err = registry.ParseReference(dstName)
			if err != nil {
				return fmt.Errorf("failed to parse ref %s: %w", dstName, err)
			}
			defaultPlatform := &ocispec.Platform{
				Architecture: cfg.Arch,
				OS:           "linux",
			}
			if tunnel != nil {
				return tunnel.Wrap(func() error {
					return copyImage(ctx, src, remoteRepo, srcName, dstName, ociConcurrency, defaultPlatform, cfg.SBOMDirectory)
				})
			}
			return copyImage(ctx, src, remoteRepo, srcName, dstName, ociConcurrency, defaultPlatform, cfg.SBOMDirectory)
		}
		pushed := []string{}
		// Delete the images that were already successfully pushed so that they aren't attempted on the next retry
		defer func() {
			for _, refInfo := range pushed {
				delete(toPush, refInfo)
			}
		}()
		for img := range toPush {
			l.Info("pushing image", "name", img)
			// If this is not a no checksum image push it for use with the Zarf agent
			if !cfg.NoChecksum {
				offlineNameCRC, err := transform.ImageTransformHost(registryURL, img)
				if err != nil {
					return err
				}

				err = retry.Do(
					func() error { return pushImage(img, offlineNameCRC) },
					retry.OnRetry(func(_ uint, err error) {
						ociConcurrency = 1
						l.Debug("retrying image push", "error", err, "concurrency", ociConcurrency)
					}),
					retry.Context(ctx),
					retry.Attempts(2),
					retry.Delay(500*time.Millisecond),
				)
				if err != nil {
					return err
				}
			}

			// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
			// (this may result in collisions but this is acceptable for this use case)
			offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, img)
			if err != nil {
				return err
			}

			err = retry.Do(
				func() error { return pushImage(img, offlineName) },
				retry.OnRetry(func(_ uint, err error) {
					ociConcurrency = 1
					l.Debug("retrying image push", "error", err, "concurrency", ociConcurrency)
				}),
				retry.Context(ctx),
				retry.Attempts(2),
				retry.Delay(500*time.Millisecond),
			)
			if err != nil {
				return err
			}

			pushed = append(pushed, img)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(uint(cfg.Retries)), retry.Delay(500*time.Millisecond), retry.OnRetry(func(attempt uint, _ error) {
		if uint(cfg.Retries) > 2 && attempt == uint(cfg.Retries)-2 {
			cfg.ResponseHeaderTimeout = 60 * time.Second // this should really never happen
		}
		l.Debug("retrying component image(s) push", "response_timeout", cfg.ResponseHeaderTimeout)
	}))
	if err != nil {
		return err
	}
	return nil
}

func addRefNameAnnotationToImages(ociLayoutDirectory string) error {
	idx, err := getIndexFromOCILayout(ociLayoutDirectory)
	if err != nil {
		return err
	}
	// Crane sets ocispec.AnnotationBaseImageName instead of ocispec.AnnotationRefName
	// which ORAS uses to find images. We do this to be backwards compatible with packages built with Crane
	var correctedManifests []ocispec.Descriptor
	for _, manifest := range idx.Manifests {
		if manifest.Annotations[ocispec.AnnotationRefName] == "" {
			manifest.Annotations[ocispec.AnnotationRefName] = manifest.Annotations[ocispec.AnnotationBaseImageName]
		}
		correctedManifests = append(correctedManifests, manifest)
	}
	idx.Manifests = correctedManifests
	err = saveIndexToOCILayout(ociLayoutDirectory, idx)
	if err != nil {
		return err
	}
	return nil
}

func copyImage(ctx context.Context, src *oci.Store, remote oras.Target, srcName string, dstName string, concurrency int, defaultPlatform *ocispec.Platform, sbomDirPath string) error {
	// Assume no platform to start as it can be nil in non container image situations
	resolveOpts := oras.DefaultResolveOptions
	desc, err := oras.Resolve(ctx, src, srcName, resolveOpts)
	if err != nil {
		return fmt.Errorf("failed to resolve image: %s: %w", srcName, err)
	}

	// If an index is pulled we should try pulling with the default platform
	if isIndex(desc.MediaType) {
		resolveOpts.TargetPlatform = defaultPlatform
		desc, err = oras.Resolve(ctx, src, srcName, resolveOpts)
		if err != nil {
			return fmt.Errorf("failed to resolve image %s with architecture %s: %w", srcName, defaultPlatform.Architecture, err)
		}
	}

	if !isManifest(desc.MediaType) {
		return fmt.Errorf("expected OCI manifest got %s", desc.MediaType)
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = concurrency
	copyOpts.WithTargetPlatform(desc.Platform)
	_, err = oras.Copy(ctx, src, srcName, remote, dstName, copyOpts)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", srcName, err)
	}

	if sbomPath, ok := findSBOM(sbomDirPath, desc); ok {
		repo, ok := remote.(*orasRemote.Repository)
		if !ok {
			return fmt.Errorf("expected *orasRemote.Repository, got %T", remote)
		}
		if supportsReferrers(ctx, repo, desc) {
			if err := pushSBOMWithReferrer(ctx, repo, desc, sbomPath); err != nil {
				return err
			}
		} else {
			if err := pushSBOMWithTag(ctx, repo, desc, sbomPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// findSBOM tries to find the SBOM for the given image
func findSBOM(srcDir string, subject ocispec.Descriptor) (string, bool) {
	// 1. gather lookup keys
	digestHex := subject.Digest.Encoded()
	refName := subject.Annotations[ocispec.AnnotationRefName] // may be ""

	// 2. build candidate filenames (.json and .sbom are both common)
	var candidates []string

	// digest-based (syft's default when run with --file <digest>.json)
	candidates = append(candidates,
		filepath.Join(srcDir, digestHex+".json"),
		filepath.Join(srcDir, digestHex+".sbom.json"),
		filepath.Join(srcDir, digestHex+".sbom"),
	)

	// ref-name-based (e.g. "alpine:3.19" -> "alpine_3.19.json")
	if refName != "" {
		sanitized := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(refName)
		candidates = append(candidates,
			filepath.Join(srcDir, sanitized+".json"),
			filepath.Join(srcDir, sanitized+".sbom.json"),
			filepath.Join(srcDir, sanitized+".sbom"),
		)
	}

	// 3. first hit wins
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}

	// 4. fall back: nothing found
	return "", false
}

// supportsReferrers probes the registry once per repo.
func supportsReferrers(ctx context.Context,
	repo *orasRemote.Repository,
	subject ocispec.Descriptor) bool {

	err := repo.Referrers(
		ctx,
		subject,
		"", // accept all artifact types
		func([]ocispec.Descriptor) error { return nil },
	)

	logger.From(ctx).Debug("checking for referrers", "error", err)

	return false
}

// convertSyftJSONFileToSPDXBytes loads the SBOM at sbomPath (expected to be
// Syft-JSON) and returns its SPDX-JSON representation.
func convertSyftJSONFileToSPDXBytes(sbomPath string) ([]byte, error) {
	f, err := os.Open(sbomPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sbomDoc, fmtObj, _, err := format.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", sbomPath, err)
	}
	if fmtObj.String() != "syft-json" {
		return nil, fmt.Errorf("file is %q, want syft-json", fmtObj.String())
	}

	enc, err := spdxjson.NewFormatEncoderWithConfig(spdxjson.DefaultEncoderConfig())
	if err != nil {
		return nil, err
	}
	outBytes, err := format.Encode(*sbomDoc, enc)
	if err != nil {
		return nil, err
	}

	return outBytes, nil
}

func pushSBOMWithReferrer(ctx context.Context, repo oras.Target, subject ocispec.Descriptor, sbomPath string) error {
	spdxBytes, err := convertSyftJSONFileToSPDXBytes(sbomPath)
	if err != nil {
		return err
	}

	mem := memory.New()
	sbomDesc := content.NewDescriptorFromBytes(
		"application/spdx+json", spdxBytes)
	if err := mem.Push(ctx, sbomDesc, bytes.NewReader(spdxBytes)); err != nil {
		return err
	}

	artDesc, err := oras.PackManifest(
		ctx,
		mem,
		oras.PackManifestVersion1_1,
		"application/spdx+json",
		oras.PackManifestOptions{
			Subject: &subject,
			Layers:  []ocispec.Descriptor{sbomDesc},
		},
	)
	if err != nil {
		return err
	}

	err = oras.CopyGraph(ctx, mem, repo, artDesc, oras.DefaultCopyGraphOptions)
	return err
}

// pushSBOMWithTag uploads an SBOM as an OCI artifact manifest and tags it
//
//	sha256-<image-digest>.sbom   (legacy cosign attach style).
func pushSBOMWithTag(ctx context.Context,
	repo oras.Target,
	subject ocispec.Descriptor,
	sbomPath string,
) error {

	tag := fmt.Sprintf("sha256-%s.sbom", subject.Digest.Encoded())

	spdxBytes, err := convertSyftJSONFileToSPDXBytes(sbomPath)
	if err != nil {
		return err
	}

	const sbomMediaType = "application/spdx+json"

	mem := memory.New()
	sbomDesc := content.NewDescriptorFromBytes(sbomMediaType, spdxBytes)
	if err := mem.Push(ctx, sbomDesc, bytes.NewReader(spdxBytes)); err != nil {
		return err
	}

	manifestDesc, err := oras.PackManifest(
		ctx, mem,
		oras.PackManifestVersion1_1, // gives us artifactType support
		sbomMediaType,               // artifactType
		oras.PackManifestOptions{
			Layers: []ocispec.Descriptor{sbomDesc},
			// Subject is intentionally nil for the “.sbom” tag pattern
		},
	)
	if err != nil {
		return err
	}

	if err := oras.CopyGraph(ctx, mem, repo, manifestDesc, oras.DefaultCopyGraphOptions); err != nil {
		return err
	}

	if err := repo.Tag(ctx, manifestDesc, tag); err != nil {
		return err
	}

	return nil
}

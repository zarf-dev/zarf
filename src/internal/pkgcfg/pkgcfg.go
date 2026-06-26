// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package pkgcfg loads and applies schema migrations to zarf.yaml files.
package pkgcfg

import (
	"context"
	"errors"
	"fmt"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// apiVersionHandler pairs a supported apiVersion with its decoder. decode returns the package
// in its native type (e.g. v1alpha1.ZarfPackage or v1beta1.Package) with any version-specific
// migrations applied;
type apiVersionHandler struct {
	version   string
	priority  int
	decode    func(ctx context.Context, node ast.Node) (any, error)
	toGeneric func(pkg any) types.Package
}

// knownAPIVersions lists every apiVersion this Zarf version can decode. To add a
// new version, append an entry with a higher priority than any existing one
var knownAPIVersions = []apiVersionHandler{
	{version: v1alpha1.APIVersion, priority: 1, decode: decodeV1Alpha1, toGeneric: v1alpha1ToGeneric},
	{version: v1beta1.APIVersion, priority: 2, decode: decodeV1Beta1, toGeneric: v1beta1ToGeneric},
}

// APIVersion reports which apiVersion a package definition should be loaded as. When the
// definition spans multiple documents, the highest-priority known version wins.
func APIVersion(b []byte) (string, error) {
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return "", err
	}

	var (
		chosen apiVersionHandler
		found  bool
	)
	for i, doc := range docs {
		version, err := apiVersionFromNode(doc.Body)
		if err != nil {
			return "", fmt.Errorf("document %d: reading apiVersion: %w", i, err)
		}
		handler, known := handlerFor(version)
		if !known {
			continue
		}
		if !found || handler.priority > chosen.priority {
			chosen = handler
			found = true
		}
	}
	if !found {
		return "", errors.New("no supported apiVersion found in package definition")
	}
	return chosen.version, nil
}

// ParseAs returns the document for the given apiVersion from a package definition that may
// contain multiple documents, decoded into its native type T.
func ParseAs[T any](ctx context.Context, b []byte, apiVersion string) (T, error) {
	var zero T
	handler, known := handlerFor(apiVersion)
	if !known {
		return zero, fmt.Errorf("unsupported apiVersion %q", apiVersion)
	}
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return zero, err
	}
	for i, doc := range docs {
		version, err := apiVersionFromNode(doc.Body)
		if err != nil {
			return zero, fmt.Errorf("document %d: reading apiVersion: %w", i, err)
		}
		docHandler, docKnown := handlerFor(version)
		if !docKnown || docHandler.version != handler.version {
			continue
		}
		native, err := handler.decode(ctx, doc.Body)
		if err != nil {
			return zero, err
		}
		out, ok := native.(T)
		if !ok {
			return zero, fmt.Errorf("decoded %q package does not match the requested type", handler.version)
		}
		return out, nil
	}
	return zero, fmt.Errorf("no %q document found in package definition", handler.version)
}

// ParseMultiDoc parses a multi doc zarf.yaml file, generally from an already built package,
// into the internal generic representation. Multi doc definitions may contain one document
// per apiVersion; the highest-priority known version wins.
func ParseMultiDoc(ctx context.Context, b []byte) (types.Package, error) {
	l := logger.From(ctx)
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return types.Package{}, err
	}

	var (
		chosen     apiVersionHandler
		chosenNode ast.Node
		found      bool
	)
	seenVersions := map[string]bool{}

	for i, doc := range docs {
		version, err := apiVersionFromNode(doc.Body)
		if err != nil {
			return types.Package{}, fmt.Errorf("document %d: reading apiVersion: %w", i, err)
		}
		handler, known := handlerFor(version)
		if !known {
			l.Debug("found unsupported API version during parse", "apiVersion", version)
			continue
		}
		if seenVersions[handler.version] {
			return types.Package{}, fmt.Errorf("duplicate apiVersion %q in package definition", handler.version)
		}
		seenVersions[handler.version] = true
		if !found || handler.priority > chosen.priority {
			chosen = handler
			chosenNode = doc.Body
			found = true
		}
	}

	if !found {
		return types.Package{}, errors.New("no supported apiVersion found in package definition")
	}
	native, err := chosen.decode(ctx, chosenNode)
	if err != nil {
		return types.Package{}, err
	}
	return chosen.toGeneric(native), nil
}

func decodeV1Alpha1(ctx context.Context, node ast.Node) (any, error) {
	var pkg v1alpha1.ZarfPackage
	if err := goyaml.NodeToValue(node, &pkg); err != nil {
		return nil, err
	}
	return internalv1alpha1.ApplyMigrations(ctx, pkg), nil
}

func v1alpha1ToGeneric(pkg any) types.Package {
	return internalv1alpha1.ConvertToGeneric(pkg.(v1alpha1.ZarfPackage)) //nolint:errcheck
}

// decodeV1Beta1 decodes a v1beta1 document into its native type. v1beta1 has no v1alpha1-style
// migrations; the conversion to the internal generic representation happens in v1beta1ToGeneric.
func decodeV1Beta1(_ context.Context, node ast.Node) (any, error) {
	var pkg v1beta1.Package
	if err := goyaml.NodeToValue(node, &pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

func v1beta1ToGeneric(pkg any) types.Package {
	return internalv1beta1.ConvertToGeneric(pkg.(v1beta1.Package)) //nolint:errcheck
}

func handlerFor(version string) (apiVersionHandler, bool) {
	if version == "" {
		version = v1alpha1.APIVersion
	}
	for _, h := range knownAPIVersions {
		if h.version == version {
			return h, true
		}
	}
	return apiVersionHandler{}, false
}

func apiVersionFromNode(node ast.Node) (string, error) {
	if node == nil {
		return "", nil
	}
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
	}
	if err := goyaml.NodeToValue(node, &probe); err != nil {
		return "", err
	}
	return probe.APIVersion, nil
}

func parseZarfYAMLDocs(b []byte) ([]*ast.DocumentNode, error) {
	file, err := parser.ParseBytes(b, 0)
	if err != nil {
		return nil, err
	}
	docs := filterEmptyDocs(file.Docs)
	if len(docs) == 0 {
		return nil, errors.New("no package definition found")
	}
	return docs, nil
}

func filterEmptyDocs(docs []*ast.DocumentNode) []*ast.DocumentNode {
	out := make([]*ast.DocumentNode, 0, len(docs))
	for _, d := range docs {
		if d == nil || d.Body == nil {
			continue
		}
		out = append(out, d)
	}
	return out
}

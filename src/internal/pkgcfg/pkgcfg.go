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

// Decoder decodes one apiVersion's document into its native package type T and converts that into
// the internal generic representation
type Decoder[T any] struct {
	version   string
	priority  int
	decode    func(ctx context.Context, node ast.Node) (T, error)
	toGeneric func(T) types.Package
}

// V1Alpha1 decodes the v1alpha1 ZarfPackage schema.
var V1Alpha1 = Decoder[v1alpha1.ZarfPackage]{
	version:   v1alpha1.APIVersion,
	priority:  1,
	decode:    decodeV1Alpha1,
	toGeneric: internalv1alpha1.ConvertToGeneric,
}

// V1Beta1 decodes the v1beta1 Package schema.
var V1Beta1 = Decoder[v1beta1.Package]{
	version:   v1beta1.APIVersion,
	priority:  2,
	decode:    decodeV1Beta1,
	toGeneric: internalv1beta1.ConvertToGeneric,
}

// knownDecoders lists every apiVersion this Zarf version can decode, type-erased for version
// selection. To add a new version, declare its Decoder above and append it here with a higher
// priority than any existing one.
var knownDecoders = []genericDecoder{
	V1Alpha1.toGenericEncoder(),
	V1Beta1.toGenericEncoder(),
}

// genericDecoder is the type-erased view of a Decoder, decoding straight to the internal generic
// representation. It is used for version selection and for ParseMultiDoc, which do not need the
// native type.
type genericDecoder struct {
	version  string
	priority int
	decode   func(ctx context.Context, node ast.Node) (types.Package, error)
}

// erase drops the native type parameter, folding decode and toGeneric into a single node→generic step.
func (d Decoder[T]) toGenericEncoder() genericDecoder {
	return genericDecoder{
		version:  d.version,
		priority: d.priority,
		decode: func(ctx context.Context, node ast.Node) (types.Package, error) {
			pkg, err := d.decode(ctx, node)
			if err != nil {
				return types.Package{}, err
			}
			return d.toGeneric(pkg), nil
		},
	}
}

// ParseAs returns the document matching the decoder's apiVersion from a package definition that may
// contain multiple documents, decoded into its native type.
func ParseAs[T any](ctx context.Context, b []byte, d Decoder[T]) (T, error) {
	var zero T
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return zero, err
	}
	for i, doc := range docs {
		version, err := apiVersionFromNode(doc.Body)
		if err != nil {
			return zero, fmt.Errorf("document %d: reading apiVersion: %w", i, err)
		}
		if normalizeAPIVersion(version) != d.version {
			continue
		}
		return d.decode(ctx, doc.Body)
	}
	return zero, fmt.Errorf("no %q document found in package definition", d.version)
}

// SelectVersion returns the apiVersion Zarf will decode from a package definition that may contain
// multiple documents; the highest-priority known version wins. Use it to pick the decode target
// before calling ParseAs.
func SelectVersion(ctx context.Context, b []byte) (string, error) {
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return "", err
	}
	d, _, err := selectDecoder(ctx, docs)
	if err != nil {
		return "", err
	}
	return d.version, nil
}

// ParseMultiDoc parses a multi doc zarf.yaml file, into the internal generic representation
// Multi doc definitions may contain one document per apiVersion; the highest-priority known version wins.
func ParseMultiDoc(ctx context.Context, b []byte) (types.Package, error) {
	docs, err := parseZarfYAMLDocs(b)
	if err != nil {
		return types.Package{}, err
	}
	d, node, err := selectDecoder(ctx, docs)
	if err != nil {
		return types.Package{}, err
	}
	return d.decode(ctx, node)
}

func decodeV1Alpha1(ctx context.Context, node ast.Node) (v1alpha1.ZarfPackage, error) {
	var pkg v1alpha1.ZarfPackage
	if err := goyaml.NodeToValue(node, &pkg); err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	pkg = internalv1alpha1.ApplyMigrations(ctx, pkg)
	pkg.Build.SetOriginalAPIVersion(v1alpha1.APIVersion)
	return pkg, nil
}

// decodeV1Beta1 decodes a v1beta1 document into its native type. v1beta1 has no v1alpha1-style
// migrations.
func decodeV1Beta1(_ context.Context, node ast.Node) (v1beta1.Package, error) {
	var pkg v1beta1.Package
	if err := goyaml.NodeToValue(node, &pkg); err != nil {
		return v1beta1.Package{}, err
	}
	pkg.Build.SetOriginalAPIVersion(v1beta1.APIVersion)
	return pkg, nil
}

// selectDecoder picks the highest-priority known apiVersion among the documents, returning its
// decoder and body node. It errors on a duplicate apiVersion or when no known version is present.
func selectDecoder(ctx context.Context, docs []*ast.DocumentNode) (genericDecoder, ast.Node, error) {
	l := logger.From(ctx)
	var (
		chosen     genericDecoder
		chosenNode ast.Node
		found      bool
	)
	seenVersions := map[string]bool{}

	for i, doc := range docs {
		version, err := apiVersionFromNode(doc.Body)
		if err != nil {
			return genericDecoder{}, nil, fmt.Errorf("document %d: reading apiVersion: %w", i, err)
		}
		d, known := decoderFor(version)
		if !known {
			l.Debug("found unsupported API version during parse", "apiVersion", version)
			continue
		}
		if seenVersions[d.version] {
			return genericDecoder{}, nil, fmt.Errorf("duplicate apiVersion %q in package definition", d.version)
		}
		seenVersions[d.version] = true
		if !found || d.priority > chosen.priority {
			chosen = d
			chosenNode = doc.Body
			found = true
		}
	}

	if !found {
		return genericDecoder{}, nil, errors.New("no supported apiVersion found in package definition")
	}
	return chosen, chosenNode, nil
}

func decoderFor(version string) (genericDecoder, bool) {
	version = normalizeAPIVersion(version)
	for _, d := range knownDecoders {
		if d.version == version {
			return d, true
		}
	}
	return genericDecoder{}, false
}

// normalizeAPIVersion treats an absent apiVersion as v1alpha1, which predates the required field.
func normalizeAPIVersion(version string) string {
	if version == "" {
		return v1alpha1.APIVersion
	}
	return version
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

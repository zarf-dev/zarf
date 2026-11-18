// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openvex/go-vex/pkg/vex"
	"github.com/stretchr/testify/require"
)

// findRepoRoot ascends directory tree to locate repository root containing go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root containing go.mod")
		}
		dir = parent
	}
}

func TestOpenVEXDocumentContract(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	vexPath := filepath.Join(repoRoot, ".vex", "zarf.cli.openvex.json")

	t.Run("vex file exists at canonical path", func(t *testing.T) {
		t.Parallel()
		info, err := os.Stat(vexPath)
		require.NoError(t, err, "expected .vex/zarf.cli.openvex.json to exist at repository root")
		require.False(t, info.IsDir(), ".vex/zarf.cli.openvex.json must be a regular file")
		require.Greater(t, info.Size(), int64(0), ".vex/zarf.cli.openvex.json must not be empty")
	})

	t.Run("vex document conforms to OpenVEX v0.2.0 schema and invariants", func(t *testing.T) {
		t.Parallel()

		// Read and unmarshal through openvex parser
		doc, err := vex.Load(vexPath)
		require.NoError(t, err, "failed to parse .vex/zarf.cli.openvex.json with go-vex")
		require.NotNil(t, doc)

		// Validate document metadata
		require.Equal(t, "https://openvex.dev/ns/v0.2.0", doc.Context, "expected OpenVEX context v0.2.0")
		require.NotEmpty(t, doc.ID, "document @id must not be empty")
		require.NotEmpty(t, doc.Author, "document author must not be empty")
		require.GreaterOrEqual(t, doc.Version, 1, "document version must be >= 1")
		require.NotNil(t, doc.Timestamp, "document timestamp must be populated")
		require.False(t, doc.Timestamp.IsZero(), "document timestamp must not be zero")
		require.True(t, doc.Timestamp.Before(time.Now().Add(24*time.Hour)), "document timestamp cannot be in the far future")

		// Validate statements
		require.NotEmpty(t, doc.Statements, "expected at least one VEX statement in document")

		seenVulns := make(map[vex.VulnerabilityID]struct{})
		for i, stmt := range doc.Statements {
			// Built-in openvex statement validation
			err := stmt.Validate()
			require.NoErrorf(t, err, "statement #%d failed openvex validation: %+v", i, stmt)

			// Required fields per OpenVEX specification
			require.NotEmptyf(t, stmt.Vulnerability.Name, "statement #%d has empty vulnerability name", i)
			seenVulns[stmt.Vulnerability.Name] = struct{}{}

			require.NotEmptyf(t, stmt.Products, "statement #%d must specify at least one product", i)
			for pi, p := range stmt.Products {
				require.NotEmptyf(t, p.ID, "statement #%d product #%d has empty product ID", i, pi)
			}

			require.Truef(t, stmt.Status.Valid(), "statement #%d has invalid status: %q", i, stmt.Status)

			// Status-specific invariants
			switch stmt.Status {
			case vex.StatusNotAffected:
				require.Truef(t, stmt.Justification.Valid(), "statement #%d (not_affected) must have a valid justification: %q", i, stmt.Justification)
				require.NotEmptyf(t, stmt.ImpactStatement, "statement #%d (not_affected) must have an impact statement explaining why vulnerability cannot be exploited", i)
			case vex.StatusAffected:
				require.NotEmptyf(t, stmt.ActionStatement, "statement #%d (affected) must provide an action statement for remediation", i)
			}

			if stmt.Timestamp != nil {
				require.Falsef(t, stmt.Timestamp.IsZero(), "statement #%d timestamp must not be zero", i)
				require.Truef(t, stmt.Timestamp.Before(time.Now().Add(24*time.Hour)), "statement #%d timestamp cannot be in the far future", i)
			}
		}

		// Verify canonical hash calculation works without error
		hash, err := doc.CanonicalHash()
		require.NoError(t, err, "failed to compute canonical hash for VEX document")
		require.NotEmpty(t, hash, "canonical hash must not be empty")
	})

	t.Run("vex document is clean JSON matching roundtrip serialization", func(t *testing.T) {
		t.Parallel()

		rawBytes, err := os.ReadFile(vexPath)
		require.NoError(t, err)

		var rawMap map[string]any
		err = json.Unmarshal(rawBytes, &rawMap)
		require.NoError(t, err, "raw .vex/zarf.cli.openvex.json must be valid JSON")

		require.Contains(t, rawMap, "@context")
		require.Contains(t, rawMap, "@id")
		require.Contains(t, rawMap, "author")
		require.Contains(t, rawMap, "version")
		require.Contains(t, rawMap, "statements")
		require.Contains(t, rawMap, "timestamp")
	})

	t.Run("init package template references canonical vex file", func(t *testing.T) {
		t.Parallel()

		v1beta1TplPath := filepath.Join(repoRoot, "packages", "init", "v1beta1", "zarf.tpl.yaml")
		content, err := os.ReadFile(v1beta1TplPath)
		require.NoError(t, err, "packages/init/v1beta1/zarf.tpl.yaml must exist")
		require.Contains(t, string(content), "zarf.cli.openvex.json: ../../../.vex/zarf.cli.openvex.json",
			"v1beta1 init template must attach repo root VEX file under attachments")

		v1alpha1Path := filepath.Join(repoRoot, "packages", "init", "v1alpha1", "zarf.yaml")
		v1alpha1Content, err := os.ReadFile(v1alpha1Path)
		require.NoError(t, err, "packages/init/v1alpha1/zarf.yaml must exist")
		require.Contains(t, string(v1alpha1Content), "zarf.cli.openvex.json: ../../../.vex/zarf.cli.openvex.json",
			"v1alpha1 init manifest must attach repo root VEX file under documentation")
	})

	t.Run("goreleaser includes vex file in checksum and release extra_files", func(t *testing.T) {
		t.Parallel()

		goreleaserPath := filepath.Join(repoRoot, ".goreleaser.yaml")
		content, err := os.ReadFile(goreleaserPath)
		require.NoError(t, err, ".goreleaser.yaml must exist")
		goreleaserYAML := string(content)

		require.Contains(t, goreleaserYAML, "./.vex/zarf.cli.openvex.json",
			".goreleaser.yaml must include .vex/zarf.cli.openvex.json in checksum extra_files and release extra_files")
	})
}

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// ValidatePackageRequirements reads <packageRoot>/REQUIREMENTS and validates agent + cluster prerequisites.
func ValidatePackageRequirements(ctx context.Context, pkgLayout *layout.PackageLayout) error {
	return ValidatePackageRequirementsFromDir(ctx, pkgLayout.DirPath())
}

// ValidatePackageRequirementsFromDir reads <dir>/REQUIREMENTS and validates agent + cluster prerequisites.
// This is useful for unit tests and for commands that inspect package contents without requiring a full PackageLayout.
func ValidatePackageRequirementsFromDir(ctx context.Context, dir string) error {
	reqPath := filepath.Join(dir, layout.Requirements)

	b, err := os.ReadFile(reqPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no REQUIREMENTS file => no-op
		}
		return fmt.Errorf("failed to read requirements.yaml: %w", err)
	}

	var rf requirementsFile
	if err := goyaml.Unmarshal(b, &rf); err != nil {
		return fmt.Errorf("failed to parse requirements.yaml: %w", err)
	}

	// Validate agent-side requirements first (fast, local).
	if rf.Agent != nil {
		if err := validateAgentRequirements(ctx, *rf.Agent); err != nil {
			return err
		}
	}

	// Cluster-side requirements: connect once if needed.
	if rf.Cluster != nil {
		c, err := cluster.New(ctx)
		if err != nil {
			return fmt.Errorf("cluster requirements exist but unable to connect to cluster: %w", err)
		}
		if err := validateClusterRequirements(ctx, c, *rf.Cluster); err != nil {
			return err
		}
	}

	logger.From(ctx).Debug("requirements validated", "file", reqPath)
	return nil
}

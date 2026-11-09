// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"context"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// DeprecationWarning is the migration notice for v1alpha1 packages.
const DeprecationWarning = "v1alpha1 packages are deprecated and do not have long-term support in Zarf v1.0; migrate to zarf.dev/v1beta1."

// WarnIfDeprecated reports v1alpha1 deprecation at package operation boundaries.
func WarnIfDeprecated(ctx context.Context, pkg api.Package) {
	if pkg.GetAPIVersion() == v1alpha1.APIVersion {
		logger.From(ctx).Warn(DeprecationWarning, "package", pkg.Metadata.Name)
	}
}

// WarnAuthorIfDeprecated reports v1alpha1 deprecation when operating on a source manifest.
func WarnAuthorIfDeprecated(ctx context.Context, pkg api.Package) {
	if pkg.GetAPIVersion() == v1alpha1.APIVersion {
		logger.From(ctx).Warn(DeprecationWarning+" See the migration guide: https://docs.zarf.dev/best-practices/v1alpha1-to-v1beta1/", "package", pkg.Metadata.Name)
	}
}

// WarnDeprecatedVariablesAndConstants reports legacy inputs after an authoring operation loads the complete package.
func WarnDeprecatedVariablesAndConstants(ctx context.Context, pkg api.Package) {
	if pkg.GetAPIVersion() != v1alpha1.APIVersion {
		return
	}
	l := logger.From(ctx)
	if len(pkg.Variables) > 0 {
		l.Warn("variables are deprecated and removed in v1beta1. Migrate to Zarf values instead", "package", pkg.Metadata.Name)
	}
	if len(pkg.Constants) > 0 {
		l.Warn("constants are deprecated and removed in v1beta1. Migrate to Zarf values instead", "package", pkg.Metadata.Name)
	}
}

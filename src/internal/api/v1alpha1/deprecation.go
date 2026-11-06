// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"context"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// WarnIfDeprecated reports v1alpha1 deprecation at package operation boundaries.
// An omitted API version is treated as v1alpha1.
func WarnIfDeprecated(ctx context.Context, apiVersion string) {
	if apiVersion == "" || apiVersion == v1alpha1.APIVersion {
		logger.From(ctx).Warn("v1alpha1 packages are deprecated and do not have long-term support in Zarf v1.0; migrate to zarf.dev/v1beta1.")
	}
}

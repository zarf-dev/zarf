// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

// The CLI registers these instead of pkg/signing so SDK consumers only link the cloud SDKs they opt into.
import (
	// Register ambient OIDC credential providers (GitHub Actions, GCP, SPIFFE, etc.)
	_ "github.com/sigstore/cosign/v3/pkg/providers/all"

	// Register the provider-specific plugins
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"
)

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"testing"

	"github.com/sigstore/sigstore/pkg/signature/kms"
	"github.com/stretchr/testify/require"
)

func TestKMSProvidersRegistered(t *testing.T) {
	providers := kms.SupportedProviders()

	for _, scheme := range []string{
		"awskms://",
		"azurekms://",
		"gcpkms://",
		"hashivault://",
		"openbao://",
	} {
		require.Contains(t, providers, scheme)
	}
}

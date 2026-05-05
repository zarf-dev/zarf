// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"os"
	"testing"

	"github.com/zarf-dev/zarf/src/pkg/feature"
)

// feature.Set is write-once across the test binary, so any feature this package's tests
// rely on is enabled here once before tests run.
func TestMain(m *testing.M) {
	if err := feature.Set([]feature.Feature{
		{Name: feature.Values, Enabled: true},
	}); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

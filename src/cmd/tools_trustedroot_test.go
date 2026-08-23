// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"reflect"
	"testing"

	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/stretchr/testify/require"
)

// knownRenames maps field names from options.TrustedRootCreateOptions to field
// names on trustedroot.CreateCmd where cosign uses different naming across the
// two structs. Add entries here when cosign renames existing fields or introduces
// new service-spec fields that follow the *Specs suffix convention.
var knownRenames = map[string]string{
	"Fulcio": "FulcioSpecs",
	"Rekor":  "RekorSpecs",
	"CTFE":   "CTFESpecs",
	"TSA":    "TSASpecs",
}

// TestTrustedRootTranslationCoverage detects drift in optionsToCreateCmd when
// cosign evolves options.TrustedRootCreateOptions or trustedroot.CreateCmd.
//
// This test will populates every exported field on the source struct with a sentinel value,
// runs the translation, and asserts the corresponding destination field received
// the sentinel. Failure modes:
//
//   - Destination field missing → cosign renamed; update knownRenames or the translation
//   - Sentinel value absent on destination → translation does not forward the field
//   - Unsupported field type → test fails so populateSentinels is updated
//
// This keeps the translation honest without needing a manual checklist on every
// cosign bump.
func TestTrustedRootTranslationCoverage(t *testing.T) {
	src := &options.TrustedRootCreateOptions{}
	populateSentinels(t, src)

	dst := optionsToCreateCmd(src)

	srcVal := reflect.ValueOf(src).Elem()
	dstVal := reflect.ValueOf(dst).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Type().Field(i)
		if !srcField.IsExported() {
			continue
		}

		dstName := srcField.Name
		if renamed, ok := knownRenames[srcField.Name]; ok {
			dstName = renamed
		}

		dstField := dstVal.FieldByName(dstName)
		require.True(t, dstField.IsValid(),
			"options.TrustedRootCreateOptions.%s has no counterpart %q on trustedroot.CreateCmd; cosign renamed a field — update knownRenames or the translation",
			srcField.Name, dstName)

		require.Equal(t, srcVal.Field(i).Interface(), dstField.Interface(),
			"options.TrustedRootCreateOptions.%s was not forwarded to trustedroot.CreateCmd.%s; update optionsToCreateCmd",
			srcField.Name, dstName)
	}
}

// populateSentinels sets every exported field on s to a distinctive non-zero value
// via reflection. Fails the test on any field type it cannot populate, so that
// cosign-introduced field types force an intentional update to this helper
// rather than silently passing.
func populateSentinels(t *testing.T, s any) {
	t.Helper()

	v := reflect.ValueOf(s).Elem()
	tp := v.Type()

	for i := 0; i < tp.NumField(); i++ {
		structField := tp.Field(i)
		if !structField.IsExported() {
			continue
		}
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}

		switch field.Kind() {
		case reflect.Bool:
			field.SetBool(true)
		case reflect.String:
			field.SetString("sentinel-" + structField.Name)
		case reflect.Slice:
			if field.Type().Elem().Kind() != reflect.String {
				t.Fatalf("populateSentinels: unsupported slice element type %s for field %s; extend helper",
					field.Type().Elem().Kind(), structField.Name)
			}
			field.Set(reflect.ValueOf([]string{"sentinel-" + structField.Name}))
		default:
			t.Fatalf("populateSentinels: unsupported field kind %s for field %s; extend helper",
				field.Kind(), structField.Name)
		}
	}
}

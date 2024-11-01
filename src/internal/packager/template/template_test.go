// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestGetSanitizedTemplateMap(t *testing.T) {
	t.Parallel()
	input := map[string]*variables.TextTemplate{
		"###ZARF_GIT_AUTH_PULL###": {Sensitive: true, Value: "secret1"},
		"###ZARF_GIT_AUTH_PUSH###": {Sensitive: true, Value: "secret2"},
		"###ZARF_REGISTRY###":      {Sensitive: false, Value: "127.0.0.1:31999"},
		"###ZARF_GIT_PUSH###":      {Sensitive: false, Value: "zarf-git-user"},
		"###ZARF_GIT_PULL###":      {Sensitive: false, Value: "zarf-git-read-user"},
	}

	expected := map[string]string{
		"###ZARF_GIT_AUTH_PULL###": "**sanitized**",
		"###ZARF_GIT_AUTH_PUSH###": "**sanitized**",
		"###ZARF_GIT_PULL###": "zarf-git-read-user",
		"###ZARF_GIT_PUSH###": "zarf-git-user",
		"###ZARF_REGISTRY###": "127.0.0.1:31999",
	}

	output := getSanitizedTemplateMap(input)
	require.Equal(t, expected, output)
}

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WithShell exposes .Shell.Quote for passing literal arguments to the selected action shell.
func (o Objects) WithShell(name string) Objects {
	o["Shell"] = shell{name: name}
	return o
}

type shell struct {
	name string
}

// Quote escapes one argument for POSIX shells or PowerShell.
func (s shell) Quote(arg string) (string, error) {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(s.name)), ".exe")
	switch name {
	case "sh", "bash", "dash", "ash", "ksh", "zsh":
		return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'", nil
	case "powershell", "pwsh":
		return "'" + strings.ReplaceAll(arg, "'", "''") + "'", nil
	default:
		return "", fmt.Errorf("argument quoting is not supported for shell %q; use a POSIX shell or PowerShell", s.name)
	}
}

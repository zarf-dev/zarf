// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

type CreateOptions struct {
	Flavor             string
	RegistryOverrides  map[string]string
	SigningKeyPath     string
	SigningKeyPassword string
	SetVariables       map[string]string
	MaxPackageSizeMB   int
	Output             string
}

func Create(ctx context.Context, packagePath string, opt CreateOptions) error {
	createOpt := layout2.CreateOptions{
		Flavor:             opt.Flavor,
		RegistryOverrides:  opt.RegistryOverrides,
		SigningKeyPath:     opt.SigningKeyPath,
		SigningKeyPassword: opt.SigningKeyPassword,
		SetVariables:       opt.SetVariables,
		MaxPackageSizeMB:   opt.MaxPackageSizeMB,
		Output:             opt.Output,
	}
	_, err := layout2.CreatePackage(ctx, packagePath, createOpt)
	if err != nil {
		return err
	}
	return nil
}

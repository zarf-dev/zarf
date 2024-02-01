// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/pkg/actions"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

func (p *Packager) runActions(defaultCfg actions.ActionDefaults, actions []actions.Action) error {
	for _, a := range actions {
		var cmdEscaped string
		var err error

		if a.Description != "" {
			cmdEscaped = a.Description
		} else {
			cmd := a.Cmd
			if a.Wait != nil {
				// TODO (@WSTARR) dessicate this code...
				if a.MaxTotalSeconds == nil {
					fiveMin := 300
					a.MaxTotalSeconds = &fiveMin
				}
				cmd, err = p.actionRunner.ConvertWaitToCmd(*a.Wait, a.MaxTotalSeconds)
				if err != nil {
					return err
				}
			}
			cmdEscaped = helpers.Truncate(cmd, 60, false)
		}

		spinner := message.NewProgressSpinner("Running \"%s\"", cmdEscaped)
		// Persist the spinner output so it doesn't get overwritten by the command output.
		spinner.EnablePreserveWrites()

		if err := p.actionRunner.RunAction(defaultCfg, a, p.variableConfig, spinner); err != nil {
			return err
		}
	}
	return nil
}

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api

// Component returns the named component.
// FIXME: probably doesn't need a separate file
func (p Package) Component(name string) (Component, bool) {
	for _, component := range p.Components {
		if component.Name == name {
			return component, true
		}
	}
	return Component{}, false
}

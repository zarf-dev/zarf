// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2023 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2023 Intevation GmbH <https://intevation.de>

package util //revive:disable-line:var-naming

// Set is a simple set type.
type Set[K comparable] map[K]struct{}

// Contains returns if the set contains a given key or not.
func (s Set[K]) Contains(k K) bool {
	_, found := s[k]
	return found
}

// Add adds a key to the set.
func (s Set[K]) Add(k K) {
	s[k] = struct{}{}
}

// Keys returns the keys of the set.
func (s Set[K]) Keys() []K {
	keys := make([]K, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

// Difference returns the differnce of two sets.
func (s Set[K]) Difference(t Set[K]) Set[K] {
	d := Set[K]{}
	for k := range s {
		if !t.Contains(k) {
			d.Add(k)
		}
	}
	return d
}

// ContainsAll returns true if all keys of a given set are in this set.
func (s Set[K]) ContainsAll(t Set[K]) bool {
	for k := range t {
		if !s.Contains(k) {
			return false
		}
	}
	return true
}

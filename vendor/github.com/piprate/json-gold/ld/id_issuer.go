// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld

import (
	"fmt"
)

// IdentifierIssuer issues unique identifiers, keeping track of any previously issued identifiers.
type IdentifierIssuer struct {
	prefix        string
	counter       int
	existing      map[string]string
	existingOrder []string
}

// NewIdentifierIssuer creates and returns a new IdentifierIssuer.
func NewIdentifierIssuer(prefix string) *IdentifierIssuer {
	return &IdentifierIssuer{
		prefix:        prefix,
		counter:       0,
		existing:      make(map[string]string),
		existingOrder: make([]string, 0),
	}
}

// Clone copies this IdentifierIssuer.
func (ii *IdentifierIssuer) Clone() *IdentifierIssuer {
	cpy := &IdentifierIssuer{
		prefix:        ii.prefix,
		counter:       ii.counter,
		existing:      make(map[string]string, len(ii.existing)),
		existingOrder: make([]string, len(ii.existingOrder)),
	}
	i := 0
	for k, v := range ii.existing {
		cpy.existing[k] = v
		cpy.existingOrder[i] = ii.existingOrder[i]
		i++
	}

	return cpy
}

// GetId Gets the new identifier for the given old identifier, where if no old
// identifier is given a new identifier will be generated.
func (ii *IdentifierIssuer) GetId(oldID string) string { //nolint:stylecheck
	if oldID != "" {
		// return existing old identifier
		if ex, present := ii.existing[oldID]; present {
			return ex
		}
	}

	id := ii.prefix + fmt.Sprintf("%d", ii.counter)
	ii.counter++

	if oldID != "" {
		ii.existing[oldID] = id
		ii.existingOrder = append(ii.existingOrder, oldID)
	}

	return id
}

// HasId returns True if the given old identifier has already been assigned a new identifier.
func (ii *IdentifierIssuer) HasId(oldID string) bool { //nolint:stylecheck
	_, hasKey := ii.existing[oldID]
	return hasKey
}

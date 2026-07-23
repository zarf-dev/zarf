// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package endpoints

// Service represents an AWS service endpoint.
type Service struct {
	id string
}

// ID returns the service endpoint's identifier.
func (s Service) ID() string {
	return s.id
}

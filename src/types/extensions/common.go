// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

import (
	"net/url"
	"reflect"
)

// ZarfComponentExtensions is a struct that contains all the official extensions
type ZarfComponentExtensions struct {
	// Big Bang Configurations
	BigBang *BigBang `json:"bigbang,omitempty" jsonschema:"description=Configurations for installing Big Bang and Flux in the cluster"`
}

type ZarfComponentExtension interface {
	LocalPaths() []string
}

func isLocal(source string) bool {
	parsedURL, err := url.Parse(source)
	if err == nil && parsedURL.Scheme == "file" {
		return true
	}
	return err == nil && parsedURL.Scheme == "" && parsedURL.Host == ""
}

func (ze ZarfComponentExtensions) LocalPaths() []string {
	var local []string

	// Loop through all the fields in the struct
	for i := 0; i < reflect.TypeOf(ze).NumField(); i++ {
		// Get the field
		field := reflect.TypeOf(ze).Field(i)

		// Get the value of the field
		value := reflect.ValueOf(ze).Field(i).Interface()

		// If the field is a pointer and the value is not nil
		if field.Type.Kind() == reflect.Ptr && !reflect.ValueOf(value).IsNil() {
			// Get the local files from the extension
			local = append(local, value.(ZarfComponentExtension).LocalPaths()...)
		}
	}

	return local
}

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

// fork from https://github.com/goccy/go-yaml/blob/master/cmd/ycat/ycat.go

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/fatih/color"
	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"
)

const yamlEscape = "\x1b"

func yamlFormat(attr color.Attribute) string {
	return fmt.Sprintf("%s[%dm", yamlEscape, attr)
}

// ColorPrintYAML pretty prints a yaml file to the console.
func ColorPrintYAML(data any) {
	text, _ := goyaml.Marshal(data)
	tokens := lexer.Tokenize(string(text))

	var p printer.Printer
	p.Bool = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiWhite),
			Suffix: yamlFormat(color.Reset),
		}
	}
	p.Number = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiWhite),
			Suffix: yamlFormat(color.Reset),
		}
	}
	p.MapKey = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiCyan),
			Suffix: yamlFormat(color.Reset),
		}
	}
	p.Anchor = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiYellow),
			Suffix: yamlFormat(color.Reset),
		}
	}
	p.Alias = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiYellow),
			Suffix: yamlFormat(color.Reset),
		}
	}
	p.String = func() *printer.Property {
		return &printer.Property{
			Prefix: yamlFormat(color.FgHiMagenta),
			Suffix: yamlFormat(color.Reset),
		}
	}

	pterm.Print(p.PrintTokens(tokens))
}

// ReadYaml reads a yaml file and unmarshals it into a given config.
func ReadYaml(path string, destConfig any) error {
	message.Debugf("Loading zarf config %s", path)
	file, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	return goyaml.Unmarshal(file, destConfig)
}

// WriteYaml writes a given config to a yaml file on disk.
func WriteYaml(path string, srcConfig any, perm fs.FileMode) error {
	// Save the parsed output to the config path given
	content, err := goyaml.Marshal(srcConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, perm)
}

// ReloadYamlTemplate marshals a given config, replaces strings and unmarshals it back.
func ReloadYamlTemplate(config any, mappings map[string]string) error {
	text, err := goyaml.Marshal(config)

	if err != nil {
		return err
	}

	for template, value := range mappings {
		// Prevent user input from escaping the trailing " during yaml marshaling
		lastIdx := len(value) - 1
		if lastIdx > -1 && string(value[lastIdx]) == "\\" {
			value = fmt.Sprintf("%s\\", value)
		}
		// Properly escape " in the yaml text output
		value = strings.ReplaceAll(value, "\"", "\\\"")
		text = []byte(strings.ReplaceAll(string(text), template, value))
	}

	return goyaml.Unmarshal(text, config)
}

// FindYamlTemplates finds strings with a given prefix in a config.
func FindYamlTemplates(config any, prefix string, suffix string) (map[string]string, error) {
	mappings := map[string]string{}

	text, err := goyaml.Marshal(config)

	if err != nil {
		return mappings, err
	}

	// Find all strings that are between the given prefix and suffix
	r := regexp.MustCompile(fmt.Sprintf("%s([A-Z_]+)%s", prefix, suffix))
	matches := r.FindAllStringSubmatch(string(text), -1)

	for _, match := range matches {
		mappings[match[1]] = ""
	}

	return mappings, nil
}

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// Source: https://github.com/argoproj/gitops-engine/blob/v0.5.2/pkg/utils/kube/kube.go#L286.
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, err
	}
	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := k8syaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %#v", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
// Source: https://github.com/argoproj/gitops-engine/blob/v0.5.2/pkg/utils/kube/kube.go#L304.
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %#v", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
}

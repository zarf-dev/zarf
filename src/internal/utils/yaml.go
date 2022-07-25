package utils

// fork from https://github.com/goccy/go-yaml/blob/master/cmd/ycat/ycat.go

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/fatih/color"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/mattn/go-colorable"
)

const yamlEscape = "\x1b"

func yamlFormat(attr color.Attribute) string {
	return fmt.Sprintf("%s[%dm", yamlEscape, attr)
}

func ColorPrintYAML(text string) {
	tokens := lexer.Tokenize(text)

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
	writer := colorable.NewColorableStdout()
	_, err := writer.Write([]byte(p.PrintTokens(tokens) + "\n"))
	if err != nil {
		message.Error(err, "Unable to print the config yaml contents")
	}
}

func ReadYaml(path string, destConfig any) error {
	message.Debugf("Loading zarf config %s", path)
	file, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	return yaml.Unmarshal(file, destConfig)
}

func WriteYaml(path string, srcConfig any, perm fs.FileMode) error {
	// Save the parsed output to the config path given
	content, err := yaml.Marshal(srcConfig)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, content, perm)
}

// ReloadYamlTemplate marshals a given config, replaces strings and unmarshals it back.
func ReloadYamlTemplate(config any, mappings map[string]string) error {
	text, err := yaml.Marshal(config)

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

	return yaml.Unmarshal(text, config)
}

// FindYamlTemplates finds strings with a given prefix in a config.
func FindYamlTemplates(config any, prefix string, suffix string) (map[string]*string, error) {
	mappings := map[string]*string{}

	text, err := yaml.Marshal(config)

	if err != nil {
		return mappings, err
	}

	// Find all strings that are between the given prefix and suffix
	r := regexp.MustCompile(fmt.Sprintf("%s([A-Z_]+)%s", prefix, suffix))
	matches := r.FindAllStringSubmatch(string(text), -1)

	for _, match := range matches {
		mappings[match[1]] = nil
	}

	return mappings, nil
}

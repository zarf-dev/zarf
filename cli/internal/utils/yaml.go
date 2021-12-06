package utils

// shamelessly stolen from https://github.com/goccy/go-yaml/blob/master/cmd/ycat/ycat.go

import (
	"fmt"
	"io/fs"
	"io/ioutil"

	"github.com/fatih/color"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
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
	_, err := writer.Write([]byte("\n\n" + p.PrintTokens(tokens) + "\n\n\n"))
	if err != nil {
		logrus.Warn("Unable to print the config yaml contents")
	}
}

func ReadYaml(path string, destConfig interface{}) error {
	logContext := logrus.WithField("path", path)
	logContext.Info("Loading dynamic config")
	file, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	return yaml.Unmarshal(file, destConfig)
}

func WriteYaml(path string, srcConfig interface{}, perm fs.FileMode) error {
	// Save the parsed output to the config path given
	content, err := yaml.Marshal(srcConfig)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, content, perm)
}

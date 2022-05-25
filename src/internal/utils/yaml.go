package utils

// fork from https://github.com/goccy/go-yaml/blob/master/cmd/ycat/ycat.go

import (
	"fmt"
	"io/fs"
	"io/ioutil"

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

package clio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
)

// Identification defines the application name and version details (generally from build information)
type Identification struct {
	Name           string `json:"application,omitempty"`    // application name
	Version        string `json:"version,omitempty"`        // application semantic version
	GitCommit      string `json:"gitCommit,omitempty"`      // git SHA at build-time
	GitDescription string `json:"gitDescription,omitempty"` // indication of git tree (either "clean" or "dirty") at build-time
	BuildDate      string `json:"buildDate,omitempty"`      // date of the build
}

type runtimeInfo struct {
	Identification
	GoVersion string `json:"goVersion,omitempty"` // go runtime version at build-time
	Compiler  string `json:"compiler,omitempty"`  // compiler used at build-time
	Platform  string `json:"platform,omitempty"`  // GOOS and GOARCH at build-time
}

type versionAddition = func() (name string, value any)

func VersionCommand(id Identification, additions ...versionAddition) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "show version information",
		Args:  cobra.NoArgs,
		// note: we intentionally do not execute through the application infrastructure (no app config is required for this command)
		RunE: func(_ *cobra.Command, _ []string) error {
			info := runtimeInfo{
				Identification: id,
				GoVersion:      runtime.Version(),
				Compiler:       runtime.Compiler,
				Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			}

			value, err := versionInfo(info, format, additions...)
			if err == nil {
				fmt.Print(value)
			}
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&format, "output", "o", "text", "the format to show the results (allowable: [text json])")

	return cmd
}

func versionInfo(info runtimeInfo, format string, additions ...versionAddition) (string, error) {
	buf := &bytes.Buffer{}

	switch format {
	case "text", "":
		type additionType struct {
			name  string
			value any
		}
		var add []additionType
		pad := 10
		for _, addition := range additions {
			name, value := addition()
			if fmt.Sprintf("%v", value) == "" {
				continue
			}
			if pad < len(name) {
				pad = len(name)
			}
			add = append(add, additionType{name: name, value: value})
		}

		appendLine(buf, "Application", pad, info.Name)
		appendLine(buf, "Version", pad, info.Version)
		appendLine(buf, "BuildDate", pad, info.BuildDate)
		appendLine(buf, "GitCommit", pad, info.GitCommit)
		appendLine(buf, "GitDescription", pad, info.GitDescription)
		appendLine(buf, "Platform", pad, info.Platform)
		appendLine(buf, "GoVersion", pad, info.GoVersion)
		appendLine(buf, "Compiler", pad, info.Compiler)

		for _, a := range add {
			appendLine(buf, a.name, pad, a.value)
		}
	case "json":
		var info any = info

		if len(additions) > 0 {
			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			err := enc.Encode(info)
			if err != nil {
				return "", fmt.Errorf("failed to show version information: %w", err)
			}

			var data map[string]any
			dec := json.NewDecoder(buf)
			err = dec.Decode(&data)
			if err != nil {
				return "", fmt.Errorf("failed to show version information: %w", err)
			}

			for _, addition := range additions {
				name, value := addition()
				name = strcase.ToLowerCamel(name)
				data[name] = value
			}

			info = data
		}

		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", " ")
		err := enc.Encode(info)
		if err != nil {
			return "", fmt.Errorf("failed to show version information: %w", err)
		}
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return buf.String(), nil
}

func appendLine(buf *bytes.Buffer, title string, width int, value any) {
	if fmt.Sprintf("%v", value) == "" {
		return
	}

	_, _ = fmt.Fprintf(buf, "%-*s %v\n", width+1, title+":", value)
}

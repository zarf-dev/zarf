// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"context"
	"errors"
	"sync"
	"testing"
	ttmpl "text/template"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestTemplateApplyMissingInput(t *testing.T) {
	for _, tt := range []struct {
		name, source, missing string
		objects               Objects
	}{
		{"missing root", `{{ .State.Registry.Address }}`, ".State.Registry.Address", Objects{}},
		{"missing value", "first line\n  {{ .Values.db.host }}", ".Values.db.host", Objects{"Values": map[string]any{"db": map[string]any{}}}},
		{"root reference", `{{ $.Values.missing }}`, ".Values.missing", Objects{"Values": map[string]any{}}},
		{"nil map", `{{ .Values.missing }}`, ".Values.missing", Objects{"Values": (map[string]any)(nil)}},
		{"alias", `{{ $v := .Values }}{{ $v.missing }}`, "$v.missing", Objects{"Values": map[string]any{}}},
		{"with", `{{ with .Values.db }}{{ .host }}{{ end }}`, ".host", Objects{"Values": map[string]any{"db": map[string]any{"port": 80}}}},
		{"range", `{{ range .Values.items }}{{ .host }}{{ end }}`, ".host", Objects{"Values": map[string]any{"items": []any{map[string]any{}}}}},
		{"parenthesized", `{{ (.Values.db).host }}`, "(.Values.db).host", Objects{"Values": map[string]any{"db": map[string]any{}}}},
		{"literal indexed alias", `{{ $v := index .Values "db" }}{{ $v.host }}`, "$v.host", Objects{"Values": map[string]any{"db": map[string]any{}}}},
		{"dynamic indexed chain", `{{ (index .Values .key).host }}`, "(index .Values .key).host", Objects{"Values": map[string]any{"db": map[string]any{}}, "key": "db"}},
		{"named template", `{{ define "nested" }}{{ .Values.missing }}{{ end }}{{ template "nested" . }}`, ".Values.missing", Objects{"Values": map[string]any{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.source)
			require.NoError(t, err)
			_, err = parsed.Apply(context.Background(), tt.objects)
			var diagnostic *Error
			require.ErrorAs(t, err, &diagnostic)
			require.Equal(t, Execution, diagnostic.Kind)
			require.Equal(t, tt.missing, diagnostic.MissingInput)
			require.Positive(t, diagnostic.Line)
			require.Positive(t, diagnostic.Column)
			var native ttmpl.ExecError
			require.ErrorAs(t, err, &native, "native execution cause must survive wrapping")
			if tt.name == "missing value" {
				require.Equal(t, 2, diagnostic.Line)
				require.Equal(t, 13, diagnostic.Column)
			}
		})
	}
}

type diagnosticMap map[string]any

func (diagnosticMap) Present() string      { return "method" }
func (diagnosticMap) Echo(s string) string { return s }
func (diagnosticMap) Child() diagnosticMap { return diagnosticMap{"value": "child"} }
func (diagnosticMap) Broken() (string, error) {
	return "", errors.New(`map has no entry for key "spoof"`)
}

type diagnosticCounter struct{ calls int }

func (c *diagnosticCounter) Next() map[string]any {
	c.calls++
	return map[string]any{"value": "once"}
}

func TestTemplateApplyExecutesOnce(t *testing.T) {
	for _, source := range []string{
		`{{ .counter.Next.value }}`,
		`{{ (.counter.Next).value }}`,
		`{{ $v := .counter.Next }}{{ $v.value }}`,
	} {
		t.Run(source, func(t *testing.T) {
			counter := &diagnosticCounter{}
			parsed, err := Parse(source)
			require.NoError(t, err)
			got, err := parsed.Apply(context.Background(), Objects{"counter": counter})
			require.NoError(t, err)
			require.Equal(t, "once", got)
			require.Equal(t, 1, counter.calls, "diagnostics must neither replay execution nor evaluate a receiver twice")
		})
	}
}

func TestTemplateApplyNativeSemantics(t *testing.T) {
	objects := Objects{
		"Values": map[string]any{
			"name": "app", "nil": nil, "typedNil": (*diagnosticMap)(nil),
			"empty": map[string]any{}, "db": map[string]any{"host": "localhost"},
			"items": []any{map[string]any{"host": "one"}, map[string]any{"host": "two"}},
		},
		"method": diagnosticMap{}, "key": "db",
		"Pkg":         v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "web", Images: []string{"image"}}}},
		"integerKeys": map[int]string{1: "one"},
		"stringMap":   map[string]string{},
	}
	for _, source := range []string{
		`{{ if false }}{{ .Values.missing }}{{ else }}ok{{ end }}`,
		`{{ if .Values.empty }}{{ .Values.missing }}{{ else }}ok{{ end }}`,
		`{{ with .Values.empty }}{{ .missing }}{{ else }}ok{{ end }}`,
		`{{ range .Values.empty }}{{ .missing }}{{ else }}ok{{ end }}`,
		`{{ and false .Values.missing }}|{{ or true .Values.missing }}`,
		`{{ with .Values.db }}{{ .host }}|{{ $.Values.name }}{{ end }}`,
		`{{ range .Values.items }}{{ .host }}{{ end }}`,
		`{{ range $i, $v := .Values.items }}{{ $i }}:{{ $v.host }}{{ break }}{{ end }}`,
		`{{ $v := .Values.db }}{{ $v.host }}{{ $v = .Values }}{{ $v.name }}`,
		`{{ .Values.nil }}|{{ .Values.nil | default "fallback" }}`,
		`{{ index .Values "absent" }}|{{ index .Values "absent" | default "fallback" }}`,
		`{{ index .stringMap "absent" }}|{{ index .stringMap "absent" | default "fallback" }}`,
		`{{ index .Values "absent" "child" }}`,
		`{{ index .Values "db" "host" }}|{{ index (index .Values .key) "host" }}`,
		`{{ (index .Values "absent").field }}`,
		`{{ .Values.nil.field }}`,
		`{{ .Values.typedNil.field }}`,
		`{{ .Values.name.field }}`,
		`{{ .integerKeys.absent }}`,
		`{{ .method.Present }}|{{ .method.Echo "hello" }}|{{ "pipeline" | .method.Echo }}`,
		`{{ .method.Child.value }}`,
		`{{ .method.Broken }}`,
		`{{ index (.Pkg.GetComponent "web").Images 0 }}`,
		`{{ (.Pkg.GetComponent "absent").Name }}`,
		`{{ .Values.absent "argument" }}`,
		`{{ "argument" | .Values.absent }}`,
		`{{ .Values.name | upper }}`,
		`{{ .Values.name | toYaml }}`,
		`{{ fail "map has no entry for key \"spoof\"" }}`,
		`{{ fail "template: str:1:3: executing \"str\" at <.Values.spoof>: map has no entry for key \"spoof\"" }}`,
		`{{ fail "sensitive payload" }}{{ .Values.missing }}`,
		`{{ upper .Values.nil }}`,
		`{{ upper .Values.absent "extra" }}`,
		`{{ (.Values.absent) "extra" }}`,
		`{{ define "empty" }}ok{{ end }}{{ template "empty" }}`,
		`{{ define "root" }}{{ . }}{{ end }}{{ template "root" .Values.nil }}`,
	} {
		t.Run(source, func(t *testing.T) {
			want, nativeErr := Apply(context.Background(), source, objects)
			parsed, err := Parse(source)
			require.NoError(t, err)
			got, err := parsed.Apply(context.Background(), objects)
			require.Equal(t, nativeErr == nil, err == nil, "native error: %v; diagnostic error: %v", nativeErr, err)
			require.Equal(t, want, got)
			if err != nil {
				var diagnostic *Error
				require.ErrorAs(t, err, &diagnostic)
				require.Equal(t, Execution, diagnostic.Kind)
				require.Empty(t, diagnostic.MissingInput, "nil, type, method, and function failures must not become missing inputs")
			}
		})
	}
}

func TestTemplateApplyIsolation(t *testing.T) {
	parsed, err := Parse(`{{ .Values.name }}`)
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 3 {
				got, err := parsed.Apply(context.Background(), Objects{"Values": map[string]any{"name": "app"}})
				if err != nil || got != "app" {
					t.Errorf("Apply() = %q, %v; want app, nil", got, err)
					return
				}
				consumers := parsed.ValueConsumers()
				if len(consumers) != 1 || len(consumers[0]) != 1 || consumers[0][0] != "name" {
					t.Errorf("ValueConsumers() = %v; want [[name]]", consumers)
				}
			}
		})
	}
	wg.Wait()
}

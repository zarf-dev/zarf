package template

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	ttmpl "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// Objects provides a map of arbitrary data to be used in the template. By convention, top level keys are capitalized
// so users can see what fields are set by the system and which are set by user input.
// Example:
// Within a template, a user can access the Values from Object{ "Values": { "app": { "name": "foo" }}}
// With {{ .Values.app.name }} => "foo"
type Objects map[string]any

// WithValues takes a value.Values and makes it available in templating Objects.
func (o Objects) WithValues(values value.Values) Objects {
	o["Values"] = values
	return o
}

// WithPackage takes a v1alpha1.ZarfPackage and makes it available in templating Objects.
func (o Objects) WithPackage(pkg v1alpha1.ZarfPackage) Objects {
	o["Constants"] = pkg.Constants
	o.WithMetadata(pkg.Metadata)
	return o
}

// WithMetadata takes the v1alpha1.ZarfMetadata section of a created package and makes it available in templating Objects.
func (o Objects) WithMetadata(meta v1alpha1.ZarfMetadata) Objects {
	o["Metadata"] = meta
	return o
}

// WithBuild takes the v1alpha1.ZarfBuildData section of a created package and makes it available in templating Objects.
func (o Objects) WithBuild(build v1alpha1.ZarfBuildData) Objects {
	o["Build"] = build
	return o
}

// WithVariables takes a variables.SetVariableMap and unwraps it into the templating Objects map so variables can be
// accessed by their key name.
func (o Objects) WithVariables(vars variables.SetVariableMap) Objects {
	m := make(map[string]string)
	for k, v := range vars {
		m[k] = v.Value
	}
	o["Variables"] = m
	return o
}

// WithConstants Takes a slice of v1alpha1.Constants and unwraps it into the templating Objects map so constants can be
// accessed in templates by their key name.
func (o Objects) WithConstants(constants []v1alpha1.Constant) Objects {
	m := make(map[string]string)
	for _, v := range constants {
		m[v.Name] = v.Value
	}
	o["Constants"] = m
	return o
}

// ApplyToFile takes a file path as well as contextual data like pkg and values, applies the context to the template,
// then writes the file back in place.
func ApplyToFile(
	ctx context.Context,
	src, dst string,
	pkg v1alpha1.ZarfPackage,
	values value.Values,
	variables variables.SetVariableMap,
	constants []v1alpha1.Constant,
) error {
	l := logger.From(ctx)
	l.Debug("applying templates in file", "path", src)
	start := time.Now()
	defer func() {
		l.Debug("finished applying templates in file", "src", src, "dst", dst, "duration", time.Since(start))
	}()

	// TODO(mkcp): Assemble this at the caller. It adds 4 params
	obj := Objects{}.
		WithValues(values).
		WithPackage(pkg).
		WithBuild(pkg.Build).
		WithVariables(variables).
		WithConstants(constants)

	// Load file into template
	tmpl, err := ttmpl.ParseFiles(src)
	if err != nil {
		return err
	}
	// Create and close destination
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(f *os.File, err error) {
		cErr := f.Close()
		if cErr != nil {
			err = fmt.Errorf("%w:%w", err, cErr)
		}
	}(f, err)
	// FIXME(mkcp): Remove stdout print this is just for checking the result in stdout
	w := io.MultiWriter(f, os.Stdout)
	// Apply template and write to destination
	if err = tmpl.Funcs(sprig.TxtFuncMap()).Execute(w, obj); err != nil {
		return err
	}
	return nil
}

// ApplyToPackageDefinition takes a ZarfPackage and a value.Values and applies templates in the package.
// TODO(mkcp): This is rly more of a proof of concept for replacing and enhancing package templates.
// FIXME(mkcp): Needless to say this needs a major refactor.
// TODO(mkcp): Rather than bumping this into Yaml and back, what we probably want to do instead is to instead load
// sections of the PackageDefinition in order. e.g. scan to metadata and parse only that section of the yaml tree.
// This would allow us to fill in metadata and then apply it to the rest of the
func ApplyToPackageDefinition(ctx context.Context, pkg v1alpha1.ZarfPackage, values value.Values) (v1alpha1.ZarfPackage, error) {
	// Create template context
	objs := Objects{}.WithValues(values).WithPackage(pkg)
	logger.From(ctx).Debug("templating package", "packageName", pkg.Metadata.Name, "templateObjects", objs)
	start := time.Now()
	defer func() {
		logger.From(ctx).Debug("done templating package", "packageName", pkg.Metadata.Name, "duration", time.Since(start))
	}()

	// Apply metadata template first
	// NOTE(mkcp): We have a two-step templating process here because Metadata can be used within templates. We need to
	// process it first and then grab the applied, plain values to store for the rest of the package definition and
	// components.
	metaYAMLBytes, err := goyaml.Marshal(pkg.Metadata)
	if err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to marshal pkg metadata to YAML: %w", err)
	}
	metaTmpl, err := ttmpl.New("package-metadata").Funcs(sprig.FuncMap()).Parse(string(metaYAMLBytes))
	if err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to parse package template: %w", err)
	}
	// Execute the template with the templateContext
	var metaBuf bytes.Buffer
	if err := metaTmpl.Execute(&metaBuf, objs); err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to execute package template: %w", err)
	}
	// Unmarshal the templated YAML back into a ZarfPackage
	var templatedMetadata v1alpha1.ZarfMetadata
	if err := goyaml.Unmarshal(metaBuf.Bytes(), &templatedMetadata); err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to unmarshal templated package: %w", err)
	}

	// Refresh objs and the Package itself with the templated metadata
	objs = objs.WithMetadata(templatedMetadata)
	pkg.Metadata = templatedMetadata

	// Now marshal the entire package to YAML
	yamlBytes, err := goyaml.Marshal(pkg)
	if err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to marshal package to YAML: %w", err)
	}

	// Create a new template and parse the YAML content
	tmpl, err := template.New("package").Funcs(sprig.FuncMap()).Parse(string(yamlBytes))
	if err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to parse package template: %w", err)
	}

	// Execute the template with the templateContext
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, objs); err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to execute package template: %w", err)
	}

	// Unmarshal the templated YAML back into a ZarfPackage
	var templatedPkg v1alpha1.ZarfPackage
	if err := goyaml.Unmarshal(buf.Bytes(), &templatedPkg); err != nil {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("failed to unmarshal templated package: %w", err)
	}

	return templatedPkg, nil
}

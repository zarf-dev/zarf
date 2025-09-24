package template

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	ttmpl "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
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

// NewObjects instantiates an Objects map, which provides templating context. The "with" options below allow for
// additional template Objects to be included.
func NewObjects(values value.Values) Objects {
	o := make(Objects)
	return o.WithValues(values)
}

// WithValues takes a value.Values and makes it available in templating Objects.
func (o Objects) WithValues(values value.Values) Objects {
	o["Values"] = values
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

// WithPackage takes a v1alpha1.ZarfPackage and makes Metadata, Constants, and Build available on the Objects map.
func (o Objects) WithPackage(pkg v1alpha1.ZarfPackage) Objects {
	// Check for fields that should be set on pkg to see if the sub-obj is available
	if pkg.Metadata.Name != "" {
		o.WithMetadata(pkg.Metadata)
	}
	if pkg.Build.User != "" {
		o.WithBuild(pkg.Build)
	}
	// Are any Constants set? Load them
	if len(pkg.Constants) > 0 {
		o.WithConstants(pkg.Constants)
	}
	return o
}

// ApplyToCmd takes a string cmd and fills in any templates.
func ApplyToCmd(ctx context.Context, cmd string, objs Objects) (string, error) {
	l := logger.From(ctx)
	l.Debug("applying templates in cmd", "cmd", cmd)

	tmpl, err := ttmpl.New("cmd").Funcs(sprig.TxtFuncMap()).Parse(cmd)
	if err != nil {
		return "", err
	}
	b := &bytes.Buffer{}
	if err = tmpl.Execute(b, objs); err != nil {
		return "", err
	}
	return b.String(), nil
}

// ApplyToFile takes a file path as well as contextual data like pkg and values, applies the context to the template,
// then writes the file back in place.
func ApplyToFile(ctx context.Context, src, dst string, objs Objects) error {
	l := logger.From(ctx)
	l.Debug("applying templates in file", "path", src)
	start := time.Now()
	defer func() {
		l.Debug("finished applying templates in file", "src", src, "dst", dst, "duration", time.Since(start))
	}()

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
	if err = tmpl.Funcs(sprig.TxtFuncMap()).Execute(w, objs); err != nil {
		return err
	}
	return nil
}

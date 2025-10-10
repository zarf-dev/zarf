// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for applying go-templates within Zarf.
package template

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	ttmpl "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

const missingKeyDefault = "missingkey=error"

var defaultFuncs = sprig.TxtFuncMap()

// Objects provides a map of arbitrary data to be used in the template. By convention, top level keys are capitalized
// so users can see what fields are set by the system and which are set by user input.
// Example:
// Within a template, a user can access the Values from Object{ "Values": { "app": { "name": "foo" }}}
// With {{ .Values.app.name }} => "foo"
type Objects map[string]any

const (
	objectKeyValues    = "Values"
	objectKeyMetadata  = "Metadata"
	objectKeyBuild     = "Build"
	objectKeyConstants = "Constants"
	objectKeyVariables = "Variables"
	objectKeyState     = "State"
)

// NewObjects instantiates an Objects map, which provides templating context. The "with" options below allow for
// additional template Objects to be included.
func NewObjects(values value.Values) Objects {
	o := make(Objects)
	return o.WithValues(values)
}

// WithValues takes a value.Values and makes it available in templating Objects.
func (o Objects) WithValues(values value.Values) Objects {
	o[objectKeyValues] = values
	return o
}

// WithMetadata takes the v1alpha1.ZarfMetadata section of a created package and makes it available in templating Objects.
func (o Objects) WithMetadata(meta v1alpha1.ZarfMetadata) Objects {
	o[objectKeyMetadata] = meta
	return o
}

// WithBuild takes the v1alpha1.ZarfBuildData section of a created package and makes it available in templating Objects.
func (o Objects) WithBuild(build v1alpha1.ZarfBuildData) Objects {
	o[objectKeyBuild] = build
	return o
}

// WithConstants Takes a slice of v1alpha1.Constants and unwraps it into the templating Objects map so constants can be
// accessed in templates by their key name.
func (o Objects) WithConstants(constants []v1alpha1.Constant) Objects {
	m := make(map[string]string)
	for _, v := range constants {
		m[v.Name] = v.Value
	}
	o[objectKeyConstants] = m
	return o
}

// WithVariables takes a variables.SetVariableMap and unwraps it into the templating Objects map so variables can be
// accessed by their key name.
func (o Objects) WithVariables(vars variables.SetVariableMap) Objects {
	m := make(map[string]string)
	for k, v := range vars {
		m[k] = v.Value
	}
	o[objectKeyVariables] = m
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

// WithState takes a state.State and makes cluster state information available in templating Objects.
// This includes registry, git, storage, artifact, and cluster configuration that's common across all components.
func (o Objects) WithState(s *state.State) Objects {
	if s == nil {
		return o
	}

	stateMap := map[string]any{
		"cluster": map[string]any{
			"appliance":    s.ZarfAppliance,
			"distro":       s.Distro,
			"architecture": s.Architecture,
		},
		"storage": map[string]any{
			"class": s.StorageClass,
		},
		"registry": map[string]any{
			"address":  s.RegistryInfo.Address,
			"nodePort": s.RegistryInfo.NodePort,
			"push": map[string]any{
				"username": s.RegistryInfo.PushUsername,
				"password": s.RegistryInfo.PushPassword,
			},
			"pull": map[string]any{
				"username": s.RegistryInfo.PullUsername,
				"password": s.RegistryInfo.PullPassword,
			},
		},
		"git": map[string]any{
			"address": s.GitServer.Address,
			"push": map[string]any{
				"username": s.GitServer.PushUsername,
				"password": s.GitServer.PushPassword,
			},
			"pull": map[string]any{
				"username": s.GitServer.PullUsername,
				"password": s.GitServer.PullPassword,
			},
		},
		"artifact": map[string]any{
			"address": s.ArtifactServer.Address,
			"push": map[string]any{
				"username": s.ArtifactServer.PushUsername,
				"token":    s.ArtifactServer.PushToken,
			},
		},
	}

	o[objectKeyState] = stateMap
	return o
}

// WithAgentState adds zarf-agent component state including TLS certificates.
func (o Objects) WithAgentState(s *state.State) Objects {
	if s == nil {
		return o
	}

	// Ensure State map exists
	stateMap, ok := o[objectKeyState].(map[string]any)
	if !ok {
		stateMap = make(map[string]any)
		o[objectKeyState] = stateMap
	}

	stateMap["agent"] = map[string]any{
		"tls": map[string]any{
			"ca":   base64.StdEncoding.EncodeToString(s.AgentTLS.CA),
			"cert": base64.StdEncoding.EncodeToString(s.AgentTLS.Cert),
			"key":  base64.StdEncoding.EncodeToString(s.AgentTLS.Key),
		},
	}

	return o
}

// WithSeedRegistryState adds seed registry and registry component state including htpasswd and secrets.
func (o Objects) WithSeedRegistryState(s *state.State) Objects {
	if s == nil {
		return o
	}

	// Ensure State map exists
	stateMap, ok := o[objectKeyState].(map[string]any)
	if !ok {
		stateMap = make(map[string]any)
		o[objectKeyState] = stateMap
	}

	// Ensure registry map exists
	registryMap, ok := stateMap["registry"].(map[string]any)
	if !ok {
		registryMap = make(map[string]any)
		stateMap["registry"] = registryMap
	}

	htpasswd, err := generateHtpasswd(&s.RegistryInfo)
	if err != nil {
		// Log error but don't fail - consistent with GetZarfTemplates behavior
		registryMap["htpasswd"] = ""
	} else {
		registryMap["htpasswd"] = htpasswd
	}
	registryMap["seed"] = fmt.Sprintf("%s:%s", helpers.IPV4Localhost, config.ZarfSeedPort)
	registryMap["secret"] = s.RegistryInfo.Secret

	return o
}

// generateHtpasswd returns an htpasswd string for the given RegistryInfo.
func generateHtpasswd(regInfo *state.RegistryInfo) (string, error) {
	// Only calculate this for internal registries to allow longer external passwords
	if regInfo.IsInternal() {
		pushUser, err := utils.GetHtpasswdString(regInfo.PushUsername, regInfo.PushPassword)
		if err != nil {
			return "", fmt.Errorf("error generating htpasswd string: %w", err)
		}

		pullUser, err := utils.GetHtpasswdString(regInfo.PullUsername, regInfo.PullPassword)
		if err != nil {
			return "", fmt.Errorf("error generating htpasswd string: %w", err)
		}

		return fmt.Sprintf("%s\\n%s", pushUser, pullUser), nil
	}

	return "", nil
}

// Apply takes a string, fills in the templates with the given Objects, and returns a new string.
func Apply(ctx context.Context, s string, objs Objects) (string, error) {
	l := logger.From(ctx)
	l.Debug("applying templates", "str", s)

	tmpl, err := ttmpl.New("str").
		Funcs(defaultFuncs).
		Option(missingKeyDefault).
		Parse(s)
	if err != nil {
		return "", err
	}
	b := &bytes.Buffer{}
	if err = tmpl.Execute(b, objs); err != nil {
		return "", err
	}
	return b.String(), nil
}

// ApplyToFile load a file path at src, fills in the templates with the given Objects, then writes the file to dst.
func ApplyToFile(ctx context.Context, src, dst string, objs Objects) error {
	l := logger.From(ctx)
	l.Debug("applying templates in file", "src", src, "dst", dst)
	start := time.Now()
	defer func() {
		l.Debug("finished applying templates in file", "src", src, "dst", dst, "duration", time.Since(start))
	}()

	tmpl, err := ttmpl.New(filepath.Base(src)).
		Funcs(defaultFuncs).
		Option(missingKeyDefault).
		ParseFiles(src)
	if err != nil {
		return err
	}

	// Create and close destination
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		cErr := f.Close()
		if cErr != nil {
			err = fmt.Errorf("%w:%w", err, cErr)
		}
	}(f)

	// Apply template and write to destination
	err = tmpl.Execute(f, objs)
	return err
}

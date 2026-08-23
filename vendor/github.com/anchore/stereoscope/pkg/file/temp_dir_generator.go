package file

import (
	"errors"
	"os"
	"strings"
)

type TempDirGenerator struct {
	rootPrefix   string
	rootLocation string
	children     []*TempDirGenerator
}

func NewTempDirGenerator(name string) *TempDirGenerator {
	return &TempDirGenerator{
		rootPrefix: name,
	}
}

func (t *TempDirGenerator) getOrCreateRootLocation() (string, error) {
	if t.rootLocation == "" {
		location, err := os.MkdirTemp("", t.rootPrefix+"-")
		if err != nil {
			return "", err
		}

		t.rootLocation = location
	}
	return t.rootLocation, nil
}

// NewGenerator creates a child generator capable of making sibling temp directories.
func (t *TempDirGenerator) NewGenerator() *TempDirGenerator {
	gen := NewTempDirGenerator(t.rootPrefix)
	t.children = append(t.children, gen)
	return gen
}

// NewDirectory creates a new temp dir within the generators prefix temp dir.
func (t *TempDirGenerator) NewDirectory(name ...string) (string, error) {
	location, err := t.getOrCreateRootLocation()
	if err != nil {
		return "", err
	}

	return os.MkdirTemp(location, strings.Join(name, "-")+"-")
}

// Cleanup deletes all temp dirs created by this generator and any child generator.
func (t *TempDirGenerator) Cleanup() error {
	var errs []error
	for _, gen := range t.children {
		if err := gen.Cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.rootLocation != "" {
		if err := os.RemoveAll(t.rootLocation); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

package file

import (
	"errors"
	"os"
	"strings"
	"sync"
)

type TempDirGenerator struct {
	// lock guards rootLocation and children; rootPrefix is write-once at construction.
	// One generator is shared by all providers in an ImageProviders() call, so concurrent
	// GetImage calls reach NewDirectory and NewGenerator at once.
	lock         sync.Mutex
	rootPrefix   string
	rootLocation string
	children     []*TempDirGenerator
}

func NewTempDirGenerator(name string) *TempDirGenerator {
	return &TempDirGenerator{
		rootPrefix: name,
	}
}

// NewGenerator creates a child generator capable of making sibling temp directories.
func (t *TempDirGenerator) NewGenerator() *TempDirGenerator {
	gen := NewTempDirGenerator(t.rootPrefix)

	t.lock.Lock()
	defer t.lock.Unlock()
	t.children = append(t.children, gen)
	return gen
}

// NewDirectory creates a new temp dir within the generators prefix temp dir.
func (t *TempDirGenerator) NewDirectory(name ...string) (string, error) {
	// the lock is held across MkdirTemp so a concurrent Cleanup cannot remove the root
	// between resolving it and creating the dir under it. This is one mkdirat, unlike
	// Cleanup's RemoveAll, so the hold is constant and does not scale with the tree.
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.rootLocation == "" {
		location, err := os.MkdirTemp("", t.rootPrefix+"-")
		if err != nil {
			return "", err
		}

		t.rootLocation = location
	}

	return os.MkdirTemp(t.rootLocation, strings.Join(name, "-")+"-")
}

// Cleanup deletes all temp dirs created by this generator and any child generator.
// The generator stays usable afterwards: a later NewDirectory starts a fresh root.
func (t *TempDirGenerator) Cleanup() error {
	// detach everything under the lock, then do the slow removal outside it. A caller that
	// races us gets a fresh root rather than a half-deleted one, and a generator attached
	// after this point is tracked for the next Cleanup instead of being orphaned.
	t.lock.Lock()
	children, rootLocation := t.children, t.rootLocation
	t.children, t.rootLocation = nil, ""
	t.lock.Unlock()

	var errs []error
	// children hold their own locks, so recurse outside of ours
	for _, gen := range children {
		if err := gen.Cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	if rootLocation != "" {
		if err := os.RemoveAll(rootLocation); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

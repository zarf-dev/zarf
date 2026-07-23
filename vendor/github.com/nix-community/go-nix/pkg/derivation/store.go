package derivation

import "context"

// Store describes the interface a Derivation store needs to implement
// to be used from here.
// Note we use pointers to Derivation structs here, so be careful modifying these.
// Look in the store/ subfolder for implementations.
type Store interface {
	// Put inserts a new Derivation into the Derivation Store.
	// All referred derivation paths should have been Put() before.
	// The resulting derivation path is returned, or an error.
	Put(context.Context, *Derivation) (string, error)

	// Get retrieves a derivation by drv path.
	// The second return argument specifies if the derivation could be found,
	// similar to how acessing from a map works.
	Get(context.Context, string) (*Derivation, error)

	// Has returns whether the derivation (by drv path) exists.
	Has(context.Context, string) (bool, error)

	// Close closes the store.
	Close() error
}

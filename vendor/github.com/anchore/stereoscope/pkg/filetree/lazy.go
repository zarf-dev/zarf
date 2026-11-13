package filetree

import (
	"fmt"
	"sync"

	"github.com/anchore/stereoscope/internal/log"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/anchore/stereoscope/pkg/tree"
)

var _ Reader = (*lazyReader)(nil)
var _ Searcher = (*lazySearcher)(nil)

// NewLazySquash returns layer squashed onto below (nil for the lowest layer), built the first time any of its
// methods is called. This lets a caller hand out a per-layer squash that may never be needed without paying to
// build it up front. Chaining one per layer, each with the previous one as below, gives every layer's squash:
// the first use of one builds it and every squash below it that has not been built yet, each exactly once.
//
// The build runs at most once, even when the reader is used from many goroutines at once: the first caller
// runs it and every other caller blocks until it is done. That first call does all of the work and cannot be
// cancelled, since none of the Reader methods take a context. below and layer are only read, never written,
// so they can be shared with other squashes and readers. The build reads them whenever it runs, so neither may
// be modified after this call.
//
// below may be nil, another squash from NewLazySquash, or any built tree: it is copied and its whiteouts are
// stripped before layer is merged on, exactly as UnionFileTree.Squash does, so a layer tree works as below
// too. layer must not be nil: a nil layer or a typed nil below panics.
//
// If the build fails it is not retried. Every method that can return an error returns that failure, now and on
// every later call, and the methods that cannot (AllFiles, AllRealPaths, HasPath and TreeReader) behave as if
// the tree were empty. The failure names the layer that broke, by its position in the chain (the lowest is 0,
// and a below that is not a lazy squash counts as 0). Every squash above it returns the same failure. It is
// logged once and returned wrapped, so compare it with errors.Is or errors.As rather than ==.
func NewLazySquash(below Reader, layer ReadWriter) Reader {
	if layer == nil {
		panic("filetree.NewLazySquash: nil layer")
	}
	if ft, ok := layer.(*FileTree); ok && ft == nil {
		panic("filetree.NewLazySquash: nil layer")
	}
	idx := 0
	if b, ok := below.(*lazyReader); ok {
		if b == nil {
			panic("filetree.NewLazySquash: typed nil below")
		}
		idx = b.idx + 1
	} else if below != nil {
		idx = 1
	}
	return newLazyReader(idx, func() (ReadWriter, error) {
		if below == nil {
			// a layer tree keeps the whiteout entries of its own changeset, which Squash strips
			u := NewUnionFileTree()
			u.PushTree(layer)
			squashed, err := u.Squash()
			if err != nil {
				return nil, fmt.Errorf("failed to squash tree %d: %w", idx, err)
			}
			return squashed, nil
		}
		lower, err := below.Copy()
		if err != nil {
			// a lower lazy squash's failure already names its layer, every layer above it fails the same way
			return nil, err
		}
		// lower is this build's own copy, so working on it in place is safe. This is what UnionFileTree.Squash
		// does with [below, layer]: strip the lowest tree's whiteouts (a no-op when below is a squash, one
		// pass over the paths next to the deep copy just made) and merge the next tree on
		if err := removeWhiteouts(lower); err != nil {
			return nil, fmt.Errorf("failed to squash tree %d: %w", idx, err)
		}
		if err := lower.Merge(layer); err != nil {
			return nil, fmt.Errorf("failed to squash tree %d: %w", idx, err)
		}
		return lower, nil
	})
}

func newLazyReader(idx int, build func() (ReadWriter, error)) *lazyReader {
	// OnceValues drops build once it has run, along with whatever it captured
	return &lazyReader{idx: idx, realize: sync.OnceValues(func() (t ReadWriter, err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("failed to squash tree %d: panicked: %v", idx, p)
			}
			if err != nil {
				// a build that returned, as is, the failure of a lazy reader below it (a layer squash
				// copying the one under it) was logged there. Anything it wrapped or joined is new
				if _, logged := err.(*loggedBuildError); !logged {
					log.WithFields("error", err).Warn("unable to build lazy file tree")
					err = &loggedBuildError{err}
				}
				t = New()
			}
		}()
		return build()
	})}
}

type lazyReader struct {
	// realize builds the tree on first use. The tree is never nil, so the methods that cannot report an
	// error always have something to delegate to.
	realize func() (ReadWriter, error)
	// idx is this squash's position in its chain, 0 for the lowest
	idx int
}

// loggedBuildError is a failed build's error once realize has logged it, so that whatever passes it on (a
// lazy reader built from this one, a search context over it) doesn't log it again.
type loggedBuildError struct{ error }

func (e *loggedBuildError) Unwrap() error { return e.error }

func (r *lazyReader) AllFiles(types ...file.Type) []file.Reference {
	t, _ := r.realize()
	return t.AllFiles(types...)
}

func (r *lazyReader) TreeReader() tree.Reader {
	t, _ := r.realize()
	return t.TreeReader()
}

func (r *lazyReader) File(path file.Path, options ...LinkResolutionOption) (bool, *file.Resolution, error) {
	t, err := r.realize()
	if err != nil {
		return false, nil, err
	}
	return t.File(path, options...)
}

func (r *lazyReader) FilesByGlob(query string, options ...LinkResolutionOption) ([]file.Resolution, error) {
	t, err := r.realize()
	if err != nil {
		return nil, err
	}
	return t.FilesByGlob(query, options...)
}

func (r *lazyReader) AllRealPaths() []file.Path {
	t, _ := r.realize()
	return t.AllRealPaths()
}

func (r *lazyReader) ListPaths(dir file.Path) ([]file.Path, error) {
	t, err := r.realize()
	if err != nil {
		return nil, err
	}
	return t.ListPaths(dir)
}

func (r *lazyReader) HasPath(path file.Path, options ...LinkResolutionOption) bool {
	t, _ := r.realize()
	return t.HasPath(path, options...)
}

func (r *lazyReader) Walk(fn func(path file.Path, f filenode.FileNode) error, conditions *WalkConditions) error {
	t, err := r.realize()
	if err != nil {
		return err
	}
	return t.Walk(fn, conditions)
}

func (r *lazyReader) Copy() (ReadWriter, error) {
	t, err := r.realize()
	if err != nil {
		return nil, err
	}
	return t.Copy()
}

// lazySearcher is what NewSearchContext returns for a lazy reader: building the link resolution index
// walks every link in the index, so it waits for the first search, which also realizes the tree.
type lazySearcher struct {
	get func() (Searcher, error)
}

func newLazySearcher(tree *lazyReader, index IndexReader) *lazySearcher {
	return &lazySearcher{get: sync.OnceValues(func() (_ Searcher, err error) {
		defer func() {
			// only a failure of the search context itself is logged here: realize logs its own
			if p := recover(); p != nil {
				err = fmt.Errorf("building lazy search context panicked: %v", p)
				log.WithFields("error", err).Warn("unable to build lazy search context")
			}
		}()
		t, err := tree.realize()
		if err != nil {
			return nil, err
		}
		return NewSearchContext(t, index), nil
	})}
}

func (s *lazySearcher) SearchByPath(path string, options ...LinkResolutionOption) (*file.Resolution, error) {
	sc, err := s.get()
	if err != nil {
		return nil, err
	}
	return sc.SearchByPath(path, options...)
}

func (s *lazySearcher) SearchByGlob(patterns string, options ...LinkResolutionOption) ([]file.Resolution, error) {
	sc, err := s.get()
	if err != nil {
		return nil, err
	}
	return sc.SearchByGlob(patterns, options...)
}

func (s *lazySearcher) SearchByMIMEType(mimeTypes ...string) ([]file.Resolution, error) {
	sc, err := s.get()
	if err != nil {
		return nil, err
	}
	return sc.SearchByMIMEType(mimeTypes...)
}

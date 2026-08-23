package file

import (
	"sort"

	"github.com/scylladb/go-set/strset"
)

// Resolution represents the fetching of a possibly non-existent file via a request path.
type Resolution struct {
	RequestPath Path
	*Reference
	// LinkResolutions represents the traversal through the filesystem to access to current reference, including all symlink and hardlink resolution.
	// note: today this only shows resolutions via the basename of the request path, but in the future it may show all resolutions.
	LinkResolutions []Resolution
}

type Resolutions []Resolution

// NewResolution create a new Resolution for the given request path, showing the resolved reference (or
// nil if it does not exist), and the link resolution of the basename of the request path transitively.
func NewResolution(path Path, ref *Reference, leafs []Resolution) *Resolution {
	return &Resolution{
		RequestPath:     path,
		Reference:       ref,
		LinkResolutions: leafs,
	}
}

func (f Resolutions) Len() int {
	return len(f)
}

func (f Resolutions) Less(i, j int) bool {
	ith := f[i]
	jth := f[j]

	ithIsReal := ith.Reference != nil && ith.RealPath == ith.RequestPath
	jthIsReal := jth.Reference != nil && jth.RealPath == jth.RequestPath

	switch {
	case ithIsReal && !jthIsReal:
		return true
	case !ithIsReal && jthIsReal:
		return false
	}

	return ith.RequestPath < jth.RequestPath
}

func (f Resolutions) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f *Resolution) HasReference() bool {
	if f == nil {
		return false
	}
	return f.Reference != nil
}

func (f *Resolution) AllPaths() []Path {
	set := strset.New()
	set.Add(string(f.RequestPath))
	if f.Reference != nil {
		set.Add(string(f.RealPath))
	}
	for _, p := range f.LinkResolutions {
		set.Add(string(p.RequestPath))
		if p.Reference != nil {
			set.Add(string(p.RealPath))
		}
	}

	paths := set.List()
	sort.Strings(paths)

	var results []Path
	for _, p := range paths {
		results = append(results, Path(p))
	}
	return results
}

func (f *Resolution) AllRequestPaths() []Path {
	set := strset.New()
	set.Add(string(f.RequestPath))
	for _, p := range f.LinkResolutions {
		set.Add(string(p.RequestPath))
	}

	paths := set.List()
	sort.Strings(paths)

	var results []Path
	for _, p := range paths {
		results = append(results, Path(p))
	}
	return results
}

// RequestResolutionPath represents the traversal through the filesystem to access to current reference, including all symlink and hardlink resolution.
func (f *Resolution) RequestResolutionPath() []Path {
	var paths []Path
	var firstPath Path
	var lastLinkResolutionIsDead bool

	if string(f.RequestPath) != "" {
		firstPath = f.RequestPath
		paths = append(paths, f.RequestPath)
	}
	for i, p := range f.LinkResolutions {
		if i == 0 && p.RequestPath == f.RequestPath {
			// ignore link resolution that starts with the same user requested path
			continue
		}
		if firstPath == "" {
			firstPath = p.RequestPath
		}

		paths = append(paths, p.RequestPath)

		if i == len(f.LinkResolutions)-1 {
			// we've reached the final link resolution
			if p.Reference == nil {
				lastLinkResolutionIsDead = true
			}
		}
	}
	if f.HasReference() && firstPath != f.RealPath && !lastLinkResolutionIsDead {
		// we've reached the final reference that was resolved
		// we should only do this if there was a link resolution
		paths = append(paths, f.RealPath)
	}
	return paths
}

// References represents the traversal through the filesystem to access to current reference, including all symlink and hardlink resolution.
func (f *Resolution) References() []Reference {
	var refs []Reference
	var lastLinkResolutionIsDead bool

	for i, p := range f.LinkResolutions {
		if p.Reference != nil {
			refs = append(refs, *p.Reference)
		}
		if i == len(f.LinkResolutions)-1 {
			// we've reached the final link resolution
			if p.Reference == nil {
				lastLinkResolutionIsDead = true
			}
		}
	}
	if f.Reference != nil && !lastLinkResolutionIsDead {
		refs = append(refs, *f.Reference)
	}
	return refs
}

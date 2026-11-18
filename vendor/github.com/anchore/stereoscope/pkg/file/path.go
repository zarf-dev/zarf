package file

import (
	"fmt"
	"path"
	"strings"
)

const (
	WhiteoutPrefix = ".wh."
	OpaqueWhiteout = WhiteoutPrefix + WhiteoutPrefix + ".opq"
	DirSeparator   = "/"
)

// Path represents a file path
type Path string

// Normalize returns the cleaned file path representation (trimmed of spaces and resolve relative notations)
func (p Path) Normalize() Path {
	// note: when normalizing we cannot trim trailing whitespace since it is valid for a path to have suffix whitespace
	var trimmed = string(p)
	if strings.Count(trimmed, " ") < len(trimmed) {
		trimmed = strings.TrimLeft(string(p), " ")
	}

	// remove trailing dir separators
	trimmed = strings.TrimRight(trimmed, DirSeparator)

	// special case for root "/"
	if trimmed == "" {
		return DirSeparator
	}
	return Path(path.Clean(trimmed))
}

func (p Path) IsAbsolutePath() bool {
	return strings.HasPrefix(string(p), DirSeparator)
}

// Basename of the path (i.e. filename)
func (p Path) Basename() string {
	return path.Base(string(p))
}

// IsDirWhiteout indicates if the path has a basename is a opaque whiteout (which means all parent directory contents should be ignored during squashing)
func (p Path) IsDirWhiteout() bool {
	return p.Basename() == OpaqueWhiteout
}

// IsWhiteout indicates if the file basename has a whiteout prefix (which means that the file should be removed during squashing)
func (p Path) IsWhiteout() bool {
	basename := p.Basename()
	return strings.HasPrefix(basename, WhiteoutPrefix) && basename != WhiteoutPrefix
}

// UnWhiteoutPath is a representation of the current path with no whiteout prefixes. A path that is not a
// whiteout is an error rather than a best-effort answer: a bare ".wh." would otherwise resolve to the parent
// directory, and at the image root that is "/", which callers then hand to a removal that refuses it.
func (p Path) UnWhiteoutPath() (Path, error) {
	if !p.IsWhiteout() {
		return "", fmt.Errorf("not a whiteout path: %q", string(p))
	}
	basename := p.Basename()
	// the opaque marker is the file named exactly ".wh..wh..opq" (as IsDirWhiteout considers it); a name that
	// merely starts with it is an ordinary whiteout of the sibling that follows the ".wh." prefix, not a
	// marker for the parent directory
	if p.IsDirWhiteout() {
		// note: this is "/" for a marker at the image root. An opaque marker is applied by removing the
		// directory's children, never by removing this path
		return p.ParentPath()
	}
	parent, err := p.ParentPath()
	if err != nil {
		return "", err
	}
	return Path(path.Join(string(parent), strings.TrimPrefix(basename, WhiteoutPrefix))), nil
}

// ParentPath returns a path object to the current files parent directory (or errors out if there is no parent)
func (p Path) ParentPath() (Path, error) {
	parent, child := path.Split(string(p))
	sanitized := Path(parent).Normalize()
	if sanitized == "/" {
		if child != "" {
			return "/", nil
		}
		return "", fmt.Errorf("no parent")
	}
	return sanitized, nil
}

// AllPaths returns all constituent paths of the current path + the current path itself (e.g. /home/wagoodman/file.txt -> /, /home, /home/wagoodman, /home/wagoodman/file.txt )
func (p Path) AllPaths() []Path {
	fullPaths := p.ConstituentPaths()
	if p != "/" {
		fullPaths = append(fullPaths, p)
	}
	return fullPaths
}

// ConstituentPaths returns all constituent paths for the current path (not including the current path itself) (e.g. /home/wagoodman/file.txt -> /, /home, /home/wagoodman )
func (p Path) ConstituentPaths() []Path {
	// every result is a prefix of one string, so memory is linear in the path length rather than in depth x length,
	// which a deep path from a tar header would otherwise blow up
	s := DirSeparator + strings.Trim(string(p), DirSeparator)
	fullPaths := []Path{DirSeparator}
	for i := 1; i < len(s); i++ {
		if s[i] == DirSeparator[0] {
			fullPaths = append(fullPaths, Path(s[:i]))
		}
	}
	return fullPaths
}

type Paths []Path

func (p Paths) Len() int           { return len(p) }
func (p Paths) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Paths) Less(i, j int) bool { return string(p[i]) < string(p[j]) }

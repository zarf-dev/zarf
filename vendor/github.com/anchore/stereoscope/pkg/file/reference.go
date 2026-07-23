package file

import "fmt"

// Reference represents a unique file. This is useful when path is not good enough (i.e. you have the same file path for two files in two different container image layers, and you need to be able to distinguish them apart)
type Reference struct {
	id       ID
	RealPath Path // file path with NO symlinks or hardlinks in constituent paths
}

// NewFileReference creates a new unique file reference for the given path.
func NewFileReference(path Path) *Reference {
	return &Reference{
		RealPath: path,
		id:       ID(nextID.Add(1)),
	}
}

// ID returns the unique ID for this file reference.
func (f *Reference) ID() ID {
	return f.id
}

// String returns a string representation of the path with a unique ID.
func (f *Reference) String() string {
	if f == nil {
		return "[nil]"
	}
	return fmt.Sprintf("[%v] real=%q", f.id, f.RealPath)
}

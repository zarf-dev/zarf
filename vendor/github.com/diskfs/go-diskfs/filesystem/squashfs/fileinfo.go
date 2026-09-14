package squashfs

// StatT is the squashfs-specific metadata returned by directoryEntry.Sys().
type StatT struct {
	// UID is the owner user ID.
	UID uint32
	// GID is the owner group ID.
	GID uint32
	// Inode is the squashfs inode number.
	Inode uint32
	// InodeType is a human-readable name for the squashfs inode type
	// (e.g. "basic-file", "extended-symlink"). It is "unknown" if the
	// inode is missing or not one of the documented types.
	InodeType string
	// Xattrs is the set of extended attributes on the entry, if any.
	Xattrs map[string]string
	// LinkTarget is the symlink target. Empty when the entry is not a symlink.
	LinkTarget string
}

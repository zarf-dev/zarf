package mtree

import (
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"github.com/sirupsen/logrus"
)

// XXX: Do we need a Difference interface to make it so people can do var x
// Difference = <something>? The main problem is that keys and inodes need to
// have different interfaces, so it's just a pain.

// DifferenceType represents the type of a discrepancy encountered for
// an object. This is also used to represent discrepancies between keys
// for objects.
type DifferenceType string

const (
	// Missing represents a discrepancy where the object is present in
	// the @old manifest but is not present in the @new manifest.
	Missing DifferenceType = "missing"

	// Extra represents a discrepancy where the object is not present in
	// the @old manifest but is present in the @new manifest.
	Extra DifferenceType = "extra"

	// Modified represents a discrepancy where the object is present in
	// both the @old and @new manifests, but one or more of the keys
	// have different values (or have not been set in one of the
	// manifests).
	Modified DifferenceType = "modified"

	// Same represents the case where two files are the same. These are
	// only generated from CompareSame().
	Same DifferenceType = "same"

	// ErrorDifference represents an attempted update to the values of
	// a keyword that failed
	ErrorDifference DifferenceType = "errored"
)

// These functions return *type from the parameter. It's just shorthand, to
// ensure that we don't accidentally expose pointers to the caller that are
// internal data.
func ePtr(e Entry) *Entry   { return &e }
func sPtr(s string) *string { return &s }

// InodeDelta Represents a discrepancy in a filesystem object between two
// DirectoryHierarchy manifests. Discrepancies are caused by entries only
// present in one manifest [Missing, Extra], keys only present in one of the
// manifests [Modified] or a difference between the keys of the same object in
// both manifests [Modified].
type InodeDelta struct {
	diff DifferenceType
	path string
	new  Entry
	old  Entry
	keys []KeyDelta
}

// Type returns the type of discrepancy encountered when comparing this inode
// between the two DirectoryHierarchy manifests.
func (i InodeDelta) Type() DifferenceType {
	return i.diff
}

// Path returns the path to the inode (relative to the root of the
// DirectoryHierarchy manifests).
func (i InodeDelta) Path() string {
	return i.path
}

// Diff returns the set of key discrepancies between the two manifests for the
// specific inode. If the DifferenceType of the inode is not Modified, then
// Diff returns nil.
func (i InodeDelta) Diff() []KeyDelta {
	return i.keys
}

// DiffPtr returns a pointer to the internal slice that would be returned by
// [InodeDelta.Diff]. This is intended to be used by tools which need to filter
// aspects of [InodeDelta] entries. If the [DifferenceType] of the inode is not
// [Modified], then DiffPtr returns nil.
func (i *InodeDelta) DiffPtr() *[]KeyDelta {
	if i.diff == Modified {
		return &i.keys
	}
	return nil
}

// Old returns the value of the inode Entry in the "old" DirectoryHierarchy (as
// determined by the ordering of parameters to Compare).
func (i InodeDelta) Old() *Entry {
	if i.diff == Modified || i.diff == Missing {
		return ePtr(i.old)
	}
	return nil
}

// New returns the value of the inode Entry in the "new" DirectoryHierarchy (as
// determined by the ordering of parameters to Compare).
func (i InodeDelta) New() *Entry {
	if i.diff == Modified || i.diff == Extra {
		return ePtr(i.new)
	}
	return nil
}

// MarshalJSON creates a JSON-encoded version of InodeDelta.
func (i InodeDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type DifferenceType `json:"type"`
		Path string         `json:"path"`
		Keys []KeyDelta     `json:"keys"`
	}{
		Type: i.diff,
		Path: i.path,
		Keys: i.keys,
	})
}

// String returns a "pretty" formatting for InodeDelta.
func (i InodeDelta) String() string {
	switch i.diff {
	case Modified:
		// Output the first failure.
		f := i.keys[0]
		return fmt.Sprintf("%q: keyword %q: expected %s; got %s", i.path, f.name, f.old, f.new)
	case Extra:
		return fmt.Sprintf("%q: unexpected path", i.path)
	case Missing:
		return fmt.Sprintf("%q: missing path", i.path)
	default:
		panic("programming error")
	}
}

// KeyDelta Represents a discrepancy in a key for a particular filesystem
// object between two DirectoryHierarchy manifests. Discrepancies are caused by
// keys only present in one manifest [Missing, Extra] or a difference between
// the keys of the same object in both manifests [Modified]. A set of these is
// returned with InodeDelta.Diff().
type KeyDelta struct {
	diff DifferenceType
	name Keyword
	old  string
	new  string
	err  error // used for update delta results
}

// Type returns the type of discrepancy encountered when comparing this key
// between the two DirectoryHierarchy manifests' relevant inode entry.
func (k KeyDelta) Type() DifferenceType {
	return k.diff
}

// Name returns the name (the key) of the KeyDeltaVal entry in the
// DirectoryHierarchy.
func (k KeyDelta) Name() Keyword {
	return k.name
}

// Old returns the value of the KeyDeltaVal entry in the "old" DirectoryHierarchy
// (as determined by the ordering of parameters to Compare). Returns nil if
// there was no entry in the "old" DirectoryHierarchy.
func (k KeyDelta) Old() *string {
	if k.diff == Modified || k.diff == Missing {
		return sPtr(k.old)
	}
	return nil
}

// New returns the value of the KeyDeltaVal entry in the "new" DirectoryHierarchy
// (as determined by the ordering of parameters to Compare). Returns nil if
// there was no entry in the "new" DirectoryHierarchy.
func (k KeyDelta) New() *string {
	if k.diff == Modified || k.diff == Extra {
		return sPtr(k.new)
	}
	return nil
}

// MarshalJSON creates a JSON-encoded version of KeyDelta.
func (k KeyDelta) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type DifferenceType `json:"type"`
		Name Keyword        `json:"name"`
		Old  string         `json:"old"`
		New  string         `json:"new"`
	}{
		Type: k.diff,
		Name: k.name,
		Old:  k.old,
		New:  k.new,
	})
}

// mapContains is just shorthand for
//
//	_, ok := m[k]
func mapContains[M ~map[K]V, K comparable, V any](m M, k K) bool {
	_, ok := m[k]
	return ok
}

// iterMapsKeys returns an iterator over all of the keys in all of the provided
// maps, with duplicate keys only being yielded once.
func iterMapsKeys[M ~map[K]V, K comparable, V any](maps ...M) iter.Seq[K] {
	seen := map[K]struct{}{}
	return func(yield func(K) bool) {
		for _, m := range maps {
			for k := range m {
				if _, ok := seen[k]; ok {
					continue
				}
				if !yield(k) {
					return
				}
				seen[k] = struct{}{}
			}
		}
	}
}

func convertToTarTime(timeVal string) (KeyVal, error) {
	var (
		timeSec, timeNsec int64
		// used to check for trailing characters
		trailing rune
	)
	n, _ := fmt.Sscanf(timeVal, "%d.%d%c", &timeSec, &timeNsec, &trailing)
	if n != 2 {
		return "", fmt.Errorf(`failed to parse "time" key: invalid format %q`, timeVal)
	}
	return KeyVal(fmt.Sprintf("tar_time=%d.%9.9d", timeSec, 0)), nil
}

// Like Compare, but for single inode entries only. Used to compute the
// cached version of inode.keys.
func compareEntry(oldEntry, newEntry Entry) ([]KeyDelta, error) {
	var (
		oldKeys = oldEntry.allKeysMap()
		newKeys = newEntry.allKeysMap()
	)

	// If both tar_time and time were specified in the set of keys, we have to
	// convert the "time" entries to "tar_time" to allow for tar archive
	// manifests to be compared with proper filesystem manifests.
	// TODO(cyphar): This really should be abstracted inside keywords.go
	if (mapContains(oldKeys, "tar_time") || mapContains(newKeys, "tar_time")) &&
		(mapContains(oldKeys, "time") || mapContains(newKeys, "time")) {

		path, _ := oldEntry.Path()
		logrus.WithFields(logrus.Fields{
			"old:tar_time": oldKeys["tar_time"],
			"new:tar_time": newKeys["tar_time"],
			"old:time":     oldKeys["time"],
			"new:time":     newKeys["time"],
		}).Debugf(`%q: "tar_time" and "time" both present`, path)

		// Clear the "time" keys.
		oldTime, oldHadTime := oldKeys["time"]
		delete(oldKeys, "time")
		newTime, newHadTime := newKeys["time"]
		delete(newKeys, "time")

		// NOTE: It is possible (though inadvisable) for a manifest to have
		// both "tar_time" and "time" set. In those cases, we favour the
		// existing "tar_time" and just ignore the "time" value.

		switch {
		case oldHadTime && !mapContains(oldKeys, "tar_time"):
			tarTime, err := convertToTarTime(oldTime.Value())
			if err != nil {
				return nil, fmt.Errorf("old entry: %w", err)
			}
			oldKeys["tar_time"] = tarTime
		case newHadTime && !mapContains(newKeys, "tar_time"):
			tarTime, err := convertToTarTime(newTime.Value())
			if err != nil {
				return nil, fmt.Errorf("new entry: %w", err)
			}
			newKeys["tar_time"] = tarTime
		}
	}

	// Are there any differences?
	var results []KeyDelta
	for k := range iterMapsKeys(newKeys, oldKeys) {
		old, oldHas := oldKeys[k]
		gnu, gnuHas := newKeys[k] // avoid shadowing "new" builtin

		switch {
		// Missing
		case !gnuHas:
			results = append(results, KeyDelta{
				diff: Missing,
				name: k,
				old:  old.Value(),
			})

		// Extra
		case !oldHas:
			results = append(results, KeyDelta{
				diff: Extra,
				name: k,
				new:  gnu.Value(),
			})

		// Modified
		default:
			if !old.Equal(gnu) {
				results = append(results, KeyDelta{
					diff: Modified,
					name: k,
					old:  old.Value(),
					new:  gnu.Value(),
				})
			}
		}
	}

	return results, nil
}

// compare is the actual workhorse for Compare() and CompareSame()
func compare(oldDh, newDh *DirectoryHierarchy, keys []Keyword, same bool) ([]InodeDelta, error) {
	toEntryMap := func(dh *DirectoryHierarchy) (map[string]Entry, error) {
		if dh == nil {
			// treat nil DirectoryHierarchy as empty
			return make(map[string]Entry), nil
		}

		// pre-allocate the map to avoid resizing
		entries := make(map[string]Entry, len(dh.Entries))
		for _, e := range dh.Entries {
			if e.Type == RelativeType || e.Type == FullType {
				path, err := e.Path()
				if err != nil {
					return nil, err
				}
				entries[path] = e
			}
		}
		return entries, nil
	}

	oldEntries, err := toEntryMap(oldDh)
	if err != nil {
		return nil, err
	}
	newEntries, err := toEntryMap(newDh)
	if err != nil {
		return nil, err
	}

	// Now we compute the diff.
	var results []InodeDelta
	for path := range iterMapsKeys(oldEntries, newEntries) {
		old, oldHas := oldEntries[path]
		gnu, gnuHas := newEntries[path] // avoid shadowing "new" builtin

		switch {
		// Missing
		case !gnuHas:
			results = append(results, InodeDelta{
				diff: Missing,
				path: path,
				old:  old,
			})

		// Extra
		case !oldHas:
			results = append(results, InodeDelta{
				diff: Extra,
				path: path,
				new:  gnu,
			})

		// Modified
		default:
			changed, err := compareEntry(old, gnu)
			if err != nil {
				return nil, fmt.Errorf("comparison failed %s: %s", path, err)
			}

			// Ignore changes to keys not in the requested set.
			if keys != nil {
				changed = slices.DeleteFunc(changed, func(delta KeyDelta) bool {
					name := delta.name.Prefix()
					return !InKeywordSlice(name, keys) &&
						// We remap time to tar_time in compareEntry, so we
						// need to treat them equivalently here.
						!(name == "time" && InKeywordSlice("tar_time", keys)) &&
						!(name == "tar_time" && InKeywordSlice("time", keys))
				})
			}

			// Check if there were any actual changes.
			if len(changed) > 0 {
				results = append(results, InodeDelta{
					diff: Modified,
					path: path,
					old:  old,
					new:  gnu,
					keys: changed,
				})
			} else if same {
				// this means that nothing changed, i.e. that
				// the files are the same.
				results = append(results, InodeDelta{
					diff: Same,
					path: path,
					old:  old,
					new:  gnu,
					keys: changed,
				})
			}
		}
	}

	return results, nil
}

// Compare compares two directory hierarchy manifests, and returns the
// list of discrepancies between the two. All of the entries in the
// manifest are considered, with differences being generated for
// RelativeType and FullType entries. Differences in structure (such as
// the way /set and /unset are written) are not considered to be
// discrepancies. The list of differences are all filesystem objects.
//
// keys controls which keys will be compared, but if keys is nil then all
// possible keys will be compared between the two manifests (allowing for
// missing entries and the like). A missing or extra key is treated as a
// Modified type.
//
// If oldDh or newDh are empty, we assume they are a hierarchy that is
// completely empty. This is purely for helping callers create synthetic
// InodeDeltas.
//
// NB: The order of the parameters matters (old, new) because Extra and
//
//	Missing are considered as different discrepancy types.
func Compare(oldDh, newDh *DirectoryHierarchy, keys []Keyword) ([]InodeDelta, error) {
	return compare(oldDh, newDh, keys, false)
}

// CompareSame is the same as Compare, except it also includes the entries
// that are the same with a Same DifferenceType.
func CompareSame(oldDh, newDh *DirectoryHierarchy, keys []Keyword) ([]InodeDelta, error) {
	return compare(oldDh, newDh, keys, true)
}

package mtree

// Check a root directory path against the DirectoryHierarchy, regarding only
// the available keywords from the list and each entry in the hierarchy.
// If keywords is nil, the check all present in the DirectoryHierarchy
//
// This is equivalent to creating a new DirectoryHierarchy with Walk(root, nil,
// keywords, fs) and then doing a Compare(dh, newDh, keywords).
func Check(root string, dh *DirectoryHierarchy, keywords []Keyword, fs FsEval) ([]InodeDelta, error) {
	if keywords == nil {
		keywords = dh.UsedKeywords()
	}

	newDh, err := Walk(root, nil, keywords, fs)
	if err != nil {
		return nil, err
	}

	return Compare(dh, newDh, keywords)
}

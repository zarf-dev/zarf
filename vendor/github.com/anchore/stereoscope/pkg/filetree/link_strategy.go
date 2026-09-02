package filetree

const (
	// followAncestorLinks deals with link resolution for all constituent paths of a given path (everything except the basename).
	// This should not be available to users but may be used internal to the package.
	followAncestorLinks LinkResolutionOption = iota

	// FollowBasenameLinks deals with link resolution for the basename of a given path (not ancestors).
	FollowBasenameLinks

	// DoNotFollowDeadBasenameLinks deals with a special case in link resolution: when a basename resolution results in
	// a dead link. This option ensures that the last link file that resolved is returned (which exists) instead of
	// the non-existing path. This is useful when the caller wants to do custom link resolution (e.g. for container
	// images: the link is dead in this layer squash, but does it resolve in a higher layer?).
	DoNotFollowDeadBasenameLinks
)

// LinkResolutionOption is a single link resolution rule.
type LinkResolutionOption int

// linkResolutionStrategy describes the full set of possible link resolution rules and their indications (to follow or not).
type linkResolutionStrategy struct {
	FollowAncestorLinks          bool
	FollowBasenameLinks          bool
	DoNotFollowDeadBasenameLinks bool
}

// newLinkResolutionStrategy creates a new linkResolutionStrategy for the given set of LinkResolutionOptions.
func newLinkResolutionStrategy(options ...LinkResolutionOption) linkResolutionStrategy {
	s := linkResolutionStrategy{}
	for _, o := range options {
		switch o {
		case FollowBasenameLinks:
			s.FollowBasenameLinks = true
		case DoNotFollowDeadBasenameLinks:
			s.DoNotFollowDeadBasenameLinks = true
		case followAncestorLinks:
			s.FollowAncestorLinks = true
		}
	}
	return s
}

// FollowLinks indicates if the current strategy supports following links in one way or another (either in path
// ancestors or basename).
func (s linkResolutionStrategy) FollowLinks() bool {
	return s.FollowAncestorLinks || s.FollowBasenameLinks
}

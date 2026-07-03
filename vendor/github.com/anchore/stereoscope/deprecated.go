package stereoscope

import (
	"slices"
	"strings"
)

// ExtractSchemeSource parses a string with any colon-delimited prefix and validates it against the set
// of known provider tags, returning a valid source name and input string to use for GetImageFromSource
//
// NOTE: since it is now possible to select which providers to use, using schemes
// in the user input text is not necessary and should be avoided due to some ambiguity this introduces
func ExtractSchemeSource(userInput string, sources ...string) (source, newInput string) {
	const SchemeSeparator = ":"
	parts := strings.SplitN(userInput, SchemeSeparator, 2)
	if len(parts) < 2 {
		return "", userInput
	}
	// the user may have provided a source hint (or this is a split from a path or docker image reference, we aren't certain yet)
	sourceHint := parts[0]
	sourceHint = strings.TrimSpace(strings.ToLower(sourceHint))
	// check the hint against the possible tags
	if slices.Contains(sources, sourceHint) {
		return sourceHint, parts[1]
	}
	// did not have any matching tags, scheme is not a valid provider scheme
	return "", userInput
}

package value

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/internal/feature"
)

type Values map[string]any

// TODO: Implement, also we're eventually going to have to schema check here.
func ParseFiles(vfs []string) (_ Values, err error) {
	m := make(map[string]any)

	// No files given
	if len(vfs) <= 0 {
		return m, nil
	}

	// Ensure feature.Values is enabled
	if !feature.IsEnabled(feature.Values) {
		return nil, fmt.Errorf("-f or --values provided but \"%s\" feature is not enabled. Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	// Ensure files exist
	err = validateFiles(vfs)
	if err != nil {
		return nil, err
	}

	for _, vf := range vfs {
		f, err := os.Open(vf)
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				err = fmt.Errorf("failed to close values file %s: %w", vf, err)
			}
		}(f)
		err = yaml.NewDecoder(f).Decode(m)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// REVIEW: Do we care about empty files? Here? Small UX tradeoff whether or not to fail on empty files
func validateFiles(vfs []string) error {
	for _, vf := range vfs {
		if _, err := os.Stat(vf); os.IsNotExist(err) {
			return fmt.Errorf("values file %s does not exist", vf)
		}
	}
	return nil
}

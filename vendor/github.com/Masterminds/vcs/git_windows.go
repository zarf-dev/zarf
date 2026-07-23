//go:build windows
// +build windows

package vcs

import (
	"os"
	"path/filepath"
	"strings"
)

func handleSubmodules(g *GitRepo, dir string) ([]byte, error) {
	// Get the submodule directories
	out, err := g.RunFromDir("git", "submodule", "foreach", "--quiet", "--recursive", "echo $sm_path")
	if err != nil {
		return out, err
	}
	cleanOut := strings.TrimSpace(string(out))
	pths := strings.Split(strings.ReplaceAll(cleanOut, "\r\n", "\n"), "\n")

	// Create the new directories. Directories are sometimes not created under
	// Windows
	for _, pth := range pths {
		fpth := filepath.Join(dir + pth)
		os.MkdirAll(fpth, 0755)
	}

	// checkout-index for each submodule. Using $path or $sm_path while iterating
	// over the submodules does not work in Windows when called from Go.
	var cOut []byte
	for _, pth := range pths {
		// Get the path to the submodule in the exported location
		fpth := EscapePathSeparator(filepath.Join(dir, pth) + string(os.PathSeparator))

		// Call checkout-index directly in the submodule rather than in the
		// parent project. This stils git submodule foreach that has trouble
		// on Windows within Go where $sm_path isn't being handled properly
		c := g.CmdFromDir("git", "checkout-index", "-f", "-a", "--prefix="+fpth)
		c.Dir = filepath.Join(c.Dir, pth)
		out, err := c.CombinedOutput()
		cOut = append(cOut, out...)
		if err != nil {
			return cOut, err
		}
	}
	return cOut, nil
}

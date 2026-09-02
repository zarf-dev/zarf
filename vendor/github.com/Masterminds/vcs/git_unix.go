//go:build !windows
// +build !windows

package vcs

import "os"

func handleSubmodules(g *GitRepo, dir string) ([]byte, error) {
	// Generate path
	path := EscapePathSeparator(dir + "$path" + string(os.PathSeparator))

	return g.RunFromDir("git", "submodule", "foreach", "--recursive", "git checkout-index -f -a --prefix="+path)
}

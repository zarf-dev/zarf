package homedir

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

const defaultCacheEnabled = true

// cacheEnabled controls whether caching is enabled.
// Note that by default, caching is enabled (true value).
var cacheEnabled atomic.Bool

// homedirCache stores the cached home directory path
var homedirCache atomic.Value

func init() {
	homedirCache.Store("")
	cacheEnabled.Store(defaultCacheEnabled)
}

// SetCacheEnable enables or disables caching of the home directory.
// By default, caching is enabled.
func SetCacheEnable(enable bool) {
	cacheEnabled.Store(enable)
}

func CacheEnabled() bool {
	return cacheEnabled.Load()
}

// Dir returns the home directory for the executing user.
//
// This uses an OS-specific method for discovering the home directory.
// An error is returned if a home directory cannot be detected.
func Dir() (string, error) {
	if cacheEnabled.Load() {
		cached := homedirCache.Load().(string)
		if cached != "" {
			return cached, nil
		}
	}

	dir, err := detectHomeDir()
	if err != nil {
		return "", err
	}

	if cacheEnabled.Load() {
		homedirCache.Store(dir)
	}
	return dir, nil
}

// detectHomeDir tries to detect the user's home directory using various methods
func detectHomeDir() (string, error) {
	// always check with the standard lib approach first
	dir, err := os.UserHomeDir()
	if err == nil && dir != "" {
		return dir, nil
	}

	// fall back to OS-specific methods
	if runtime.GOOS == "windows" {
		return dirWindows()
	}
	return dirUnix(runtime.GOOS)
}

// Expand expands the path to include the home directory if the path
// is prefixed with `~`. If it isn't prefixed with `~`, the path is
// returned as-is.
func Expand(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] != '~' {
		return path, nil
	}

	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		return "", errors.New("cannot expand user-specific home dir")
	}

	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path[1:]), nil
}

// Reset clears the cache, forcing the next call to Dir to re-detect
// the home directory. This generally never has to be called, but can be
// useful in tests if you're modifying the home directory via the HOME
// env var or something.
func Reset() {
	homedirCache.Store("")
}

func dirUnix(goos string) (string, error) {
	homeEnv := "HOME"
	if goos == "plan9" {
		// on plan9, env vars are lowercase.
		homeEnv = "home"
	}

	// first prefer the HOME environmental variable
	if home := os.Getenv(homeEnv); home != "" {
		return home, nil
	}

	var stdout bytes.Buffer

	// if that fails, try OS specific commands
	if goos == "darwin" {
		cmd := exec.Command("sh", "-c", `dscl -q . -read /Users/"$(whoami)" NFSHomeDirectory | sed 's/^[^ ]*: //'`)
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil {
			result := strings.TrimSpace(stdout.String())
			if result != "" {
				return result, nil
			}
		}
	} else {
		cmd := exec.Command("getent", "passwd", strconv.Itoa(os.Getuid())) //nolint:gosec
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			// if the error is ErrNotFound, we ignore it. Otherwise, return it.
			if !errors.Is(err, exec.ErrNotFound) {
				return "", err
			}
		} else {
			if passwd := strings.TrimSpace(stdout.String()); passwd != "" {
				// username:password:uid:gid:gecos:home:shell
				passwdParts := strings.SplitN(passwd, ":", 7)
				if len(passwdParts) > 5 {
					return passwdParts[5], nil
				}
			}
		}
	}

	// if all else fails, try the shell
	stdout.Reset()
	cmd := exec.Command("sh", "-c", "cd && pwd")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", errors.New("blank output when reading home directory")
	}

	return result, nil
}

func dirWindows() (string, error) {
	// first prefer the HOME environmental variable
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}

	// prefer standard environment variable USERPROFILE
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home, nil
	}

	drive := os.Getenv("HOMEDRIVE")
	path := os.Getenv("HOMEPATH")
	home := drive + path
	if drive == "" || path == "" {
		return "", errors.New("HOMEDRIVE, HOMEPATH, or USERPROFILE are blank")
	}

	return home, nil
}

package projectroot

import (
	"errors"
	"os"
	"path/filepath"
)

const MarkerDirName = ".knowledger"

// Discover walks up from the current working directory looking for a project marker.
func Discover() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	return DiscoverFrom(cwd)
}

// DiscoverFrom is Discover with the starting directory parameterised.
// It stops at the filesystem root and at the user's home directory
// (so `~/` is not treated as a project root).
func DiscoverFrom(start string) (string, bool, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false, err
	}
	home, _ := os.UserHomeDir()
	dir := abs
	for {
		if home != "" && dir == home {
			return "", false, nil
		}
		found, err := hasProjectMarker(dir)
		if err != nil {
			return "", false, err
		}
		if found {
			return dir, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}

func hasProjectMarker(dir string) (bool, error) {
	for _, name := range []string{MarkerDirName, ".git"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil {
			return info.IsDir(), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ErrBaseDirMissing matches a BaseDirMissingError.
var ErrBaseDirMissing = errors.New("basedir does not exist")

// BaseDirMissingError reports that a basedir's directory does not exist, as
// opposed to a basedir that exists and has no projects yet.
type BaseDirMissingError struct {
	Path string
}

func (err *BaseDirMissingError) Error() string {
	return fmt.Sprintf("basedir %s does not exist", err.Path)
}

func (err *BaseDirMissingError) Is(target error) bool {
	return target == ErrBaseDirMissing
}

// ProjectNames lists the projects in baseDir, sorted: the directories under
// its projects directory whose names are valid path elements. A basedir
// without a projects directory has none and gives nil; one whose projects
// directory is empty, for example after its last project was deleted, gives
// an empty list. A basedir that does not exist is a
// BaseDirMissingError; each caller decides what that means for it, which is
// why this is the one place that reads the projects directory.
func ProjectNames(baseDir string) ([]string, error) {
	info, err := os.Stat(baseDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, &BaseDirMissingError{Path: baseDir}
	}
	if err != nil {
		return nil, fmt.Errorf("read basedir %s: %w", baseDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("basedir %s is not a directory", baseDir)
	}
	entries, err := os.ReadDir(filepath.Join(baseDir, "projects"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list projects in %s: %w", baseDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && IsValidPathElement(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// RequireBaseDir returns a BaseDirMissingError if baseDir does not exist.
// A command that only reads state calls it for a basedir that was given to
// it, which is most likely mistyped if missing. The implicit default may not
// exist yet: that is a basedir in which nothing has run.
func RequireBaseDir(baseDir string) error {
	if _, err := os.Stat(baseDir); errors.Is(err, os.ErrNotExist) {
		return &BaseDirMissingError{Path: baseDir}
	}
	return nil
}

// ProjectNamesOrNone is ProjectNames for a basedir that may be the
// implicit default before its first run, or a basedir already checked by
// RequireBaseDir: a missing basedir has no projects.
func ProjectNamesOrNone(baseDir string) ([]string, error) {
	names, err := ProjectNames(baseDir)
	if errors.Is(err, ErrBaseDirMissing) {
		return nil, nil
	}
	return names, err
}

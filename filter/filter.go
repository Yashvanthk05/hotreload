package filter

import (
	"path/filepath"
	"strings"
)

// ignoredDirs are directory names that should never be watched or trigger rebuilds.
var ignoredDirs = []string{
	".git",
	"node_modules",
	"vendor",
	".idea",
	".vscode",
	".hg",
	".svn",
	"bin",
}

// ignoredExts are file extensions that should be ignored.
var ignoredExts = []string{
	".swp",
	".tmp",
	".test",
	".exe",
	".dll",
	".so",
	".dylib",
	".o",
	".a",
}

// ignoredNames are exact filenames to ignore.
var ignoredNames = []string{
	".DS_Store",
	"Thumbs.db",
}

// ShouldIgnore returns true if the given path should NOT trigger a rebuild.
// It checks the path against known ignored directories, file extensions,
// file name prefixes/suffixes, and exact names.
func ShouldIgnore(path string) bool {
	// Normalize the path separators
	path = filepath.ToSlash(path)

	// Check each component of the path for ignored directories
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		for _, dir := range ignoredDirs {
			if part == dir {
				return true
			}
		}
	}

	base := filepath.Base(path)

	// Check exact names
	for _, name := range ignoredNames {
		if base == name {
			return true
		}
	}

	// Check file name patterns (prefix/suffix)
	// Vim/emacs swap files: *~, *.swp, #*#
	if strings.HasSuffix(base, "~") {
		return true
	}
	if strings.HasPrefix(base, "#") && strings.HasSuffix(base, "#") {
		return true
	}
	// Dot-prefixed hidden files that editors create temporarily
	if strings.HasPrefix(base, ".#") {
		return true
	}

	// Check extensions
	ext := filepath.Ext(base)
	for _, ignoredExt := range ignoredExts {
		if ext == ignoredExt {
			return true
		}
	}

	return false
}

// IsIgnoredDir returns true if the given directory name or path component
// is in the list of directories that should never be watched.
func IsIgnoredDir(name string) bool {
	base := filepath.Base(name)
	for _, dir := range ignoredDirs {
		if base == dir {
			return true
		}
	}
	return false
}

package filter

import (
	"path/filepath"
	"strings"
)

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

var ignoredNames = []string{
	".DS_Store",
	"Thumbs.db",
}

func ShouldIgnore(path string) bool {
	path = filepath.ToSlash(path)

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

	for _, name := range ignoredNames {
		if base == name {
			return true
		}
	}

	if strings.HasSuffix(base, "~") {
		return true
	}
	if strings.HasPrefix(base, "#") && strings.HasSuffix(base, "#") {
		return true
	}

	if strings.HasPrefix(base, ".#") {
		return true
	}

	ext := filepath.Ext(base)
	for _, ignoredExt := range ignoredExts {
		if ext == ignoredExt {
			return true
		}
	}

	return false
}

func IsIgnoredDir(name string) bool {
	base := filepath.Base(name)
	for _, dir := range ignoredDirs {
		if base == dir {
			return true
		}
	}
	return false
}

package filter_test

import (
	"testing"

	"hotreload/filter"
)

func TestShouldIgnore(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		ignore bool
	}{
		// Should be ignored
		{".git directory", "/project/.git/config", true},
		{".git file in subdir", "/project/.git/HEAD", true},
		{"node_modules", "/project/node_modules/lodash/index.js", true},
		{"vendor dir", "/project/vendor/github.com/foo/bar.go", true},
		{".idea dir", "/project/.idea/workspace.xml", true},
		{".vscode dir", "/project/.vscode/settings.json", true},
		{"vim swap file", "/project/main.go.swp", true},
		{"vim swap extension", "/project/.main.go.swp", true},
		{"tilde backup file", "/project/main.go~", true},
		{"emacs lock file", "/project/#main.go#", true},
		{"emacs auto-save", "/project/.#main.go", true},
		{"tmp extension", "/project/build.tmp", true},
		{".DS_Store", "/project/.DS_Store", true},
		{".test binary", "/project/foo.test", true},

		// Should NOT be ignored
		{"regular .go file", "/project/main.go", false},
		{"nested .go file", "/project/internal/server/handler.go", false},
		{"go.mod", "/project/go.mod", false},
		{"go.sum", "/project/go.sum", false},
		{"Makefile", "/project/Makefile", false},
		{"README.md", "/project/README.md", false},
		{"testserver .go", "/project/testserver/main.go", false},
		{"yaml config", "/project/config.yaml", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filter.ShouldIgnore(tc.path)
			if got != tc.ignore {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.path, got, tc.ignore)
			}
		})
	}
}

func TestIsIgnoredDir(t *testing.T) {
	cases := []struct {
		dir    string
		ignore bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{"/project/.git", true},
		{"/project/node_modules", true},
		{"src", false},
		{"internal", false},
		{"cmd", false},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			got := filter.IsIgnoredDir(tc.dir)
			if got != tc.ignore {
				t.Errorf("IsIgnoredDir(%q) = %v, want %v", tc.dir, got, tc.ignore)
			}
		})
	}
}

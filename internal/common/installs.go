package common

import (
	"os"
	"path/filepath"
)

// Install is one executable named after a tool, found on $PATH or in a
// sat-owned directory.
type Install struct {
	// Source is the source type the binary's location belongs to.
	Source string
	// Path is where the executable was found.
	Path string
	// Real is Path with symlinks resolved.
	Real string
	// Active marks the install a shell resolves the name to.
	Active bool
}

// FindInstalls lists every distinct executable named tool: each $PATH
// directory in order (the first hit is the active one), then the sat-owned
// and go bin directories, which may hold an install whose name is taken on
// $PATH. Installs resolving to the same file are reported once.
func FindInstalls(tool string) []Install {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	pathDirs := len(dirs)
	dirs = append(dirs, AppImagesDir(), FlatpakWrapperDir(), HuberBinDir(), GoBinDir())

	var installs []Install
	seen := map[string]bool{}
	hasActive := false
	for i, dir := range dirs {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, tool)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&binFileMode == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if seen[real] {
			continue
		}
		seen[real] = true

		active := i < pathDirs && !hasActive
		hasActive = hasActive || active
		installs = append(installs, Install{Source: ClassifyPath(real), Path: path, Real: real, Active: active})
	}
	return installs
}

// NaturalName resolves a name that is a sat-owned bin link to the natural
// name of its target (basename of the link target); any other name is
// returned unchanged.
func NaturalName(name string) string {
	links, err := localBinLinks()
	if err != nil {
		return name
	}
	for _, l := range links {
		if l.name != name {
			continue
		}
		if _, ok := ownerSuffix(filepath.Dir(l.target)); ok {
			return filepath.Base(l.target)
		}
	}
	return name
}

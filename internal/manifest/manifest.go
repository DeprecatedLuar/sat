package manifest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// Manifest file format constants
	ManifestDelimiter = "="
	ManifestFieldCount = 2  // tool=source
	CommentPrefix     = "#"

	// File permissions
	DirPermissions  = 0755
	FilePermissions = 0644
)

var manifestMutex sync.Mutex

// EnsureManifest creates the manifest file and parent directories if they don't exist
func EnsureManifest(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create manifest: %w", err)
		}
		f.Close()
	}
	return nil
}

// entry is a single tool=source manifest line, kept in file order
type entry struct {
	tool   string
	source string
}

// readEntries reads the manifest file into an ordered slice of entries.
// Missing files are treated as empty (no entries), matching prior Get/Has behavior.
func readEntries(path string) ([]entry, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}
		parts := strings.SplitN(line, ManifestDelimiter, ManifestFieldCount)
		if len(parts) == ManifestFieldCount {
			entries = append(entries, entry{tool: parts[0], source: parts[1]})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// writeEntries writes entries to the manifest file in the given order
func writeEntries(path string, entries []entry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range entries {
		fmt.Fprintf(f, "%s%s%s\n", e.tool, ManifestDelimiter, e.source)
	}
	return nil
}

// Add adds a tool to the system manifest
// Format: tool=source:identity:version
// Existing tools are updated in place (position preserved); new tools are appended.
func Add(tool, source string) error {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	if err := EnsureManifest(path); err != nil {
		return err
	}

	entries, err := readEntries(path)
	if err != nil {
		return err
	}

	found := false
	for i := range entries {
		if entries[i].tool == tool {
			entries[i].source = source
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry{tool: tool, source: source})
	}

	return writeEntries(path, entries)
}

// Get retrieves the source string for a tool
func Get(tool string) string {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	entries, err := readEntries(ManifestPath())
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.tool == tool {
			return e.source
		}
	}
	return ""
}

// Has checks if a tool exists in the manifest
func Has(tool string) bool {
	return Get(tool) != ""
}

// Remove removes a tool from the manifest
func Remove(tool string) error {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	entries, err := readEntries(path)
	if err != nil {
		return err
	}

	filtered := entries[:0]
	for _, e := range entries {
		if e.tool != tool {
			filtered = append(filtered, e)
		}
	}

	return writeEntries(path, filtered)
}

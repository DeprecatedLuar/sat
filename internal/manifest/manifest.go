package manifest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	// Manifest file format constants
	ManifestDelimiter  = "="
	ManifestFieldCount = 2 // tool=source
	CommentPrefix      = "#"

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

// writeEntries writes entries to the manifest file in the given order.
// Writes to a temp file in the same directory then renames over path, so a
// concurrent reader never observes a truncated manifest (rename(2) is
// atomic within one filesystem) - readEntries otherwise cannot distinguish
// a torn write from a legitimately smaller manifest.
func writeEntries(path string, entries []entry) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".manifest-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	for _, e := range entries {
		if _, err := fmt.Fprintf(tmp, "%s%s%s\n", e.tool, ManifestDelimiter, e.source); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, FilePermissions); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
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

// AddMany applies several tool=source updates in a single read-modify-write,
// so N drift corrections cost one manifest rewrite instead of N. Per-entry
// semantics match Add: an existing tool is updated in place (position
// preserved), a tool not yet tracked is appended. New tools are appended in
// sorted order so the result is deterministic. A tool already recording the
// given source is left untouched and does not count toward the returned
// total; when nothing actually changes, no write is performed at all.
// Returns the number of entries changed.
func AddMany(updates map[string]string) (int, error) {
	if len(updates) == 0 {
		return 0, nil
	}

	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	if err := EnsureManifest(path); err != nil {
		return 0, err
	}

	entries, err := readEntries(path)
	if err != nil {
		return 0, err
	}

	remaining := make(map[string]string, len(updates))
	for tool, source := range updates {
		remaining[tool] = source
	}

	changed := 0
	for i := range entries {
		source, ok := remaining[entries[i].tool]
		if !ok {
			continue
		}
		delete(remaining, entries[i].tool)
		if entries[i].source == source {
			continue
		}
		entries[i].source = source
		changed++
	}

	if len(remaining) > 0 {
		newTools := make([]string, 0, len(remaining))
		for tool := range remaining {
			newTools = append(newTools, tool)
		}
		sort.Strings(newTools)
		for _, tool := range newTools {
			entries = append(entries, entry{tool: tool, source: remaining[tool]})
			changed++
		}
	}

	if changed == 0 {
		return 0, nil
	}

	return changed, writeEntries(path, entries)
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

// Entry is a single tool=source manifest record, in file order
type Entry struct {
	Tool   string
	Source string
}

// All returns every entry in the system manifest, in file order
func All() ([]Entry, error) {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	raw, err := readEntries(ManifestPath())
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, len(raw))
	for i, e := range raw {
		entries[i] = Entry{Tool: e.tool, Source: e.source}
	}
	return entries, nil
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

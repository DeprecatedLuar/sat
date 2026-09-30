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

// Entry is a single tool=source manifest record, in file order. The pair
// (Tool, canonical source type) is its identity.
type Entry struct {
	Tool   string
	Source string
}

// sourceTypeAliases maps source types recorded under a legacy or per-OS name
// to the canonical type they are the same ecosystem as. "nixos" is
// deliberately absent: declarative NixOS packages are distinct from nix profile
// installs.
var sourceTypeAliases = map[string]string{
	"rust":   "cargo",
	"github": "gh",
	"apt":    "system",
	"pacman": "system",
	"apk":    "system",
	"dnf":    "system",
}

// CanonicalSourceType resolves a source type to the canonical form used to
// compare manifest identities.
func CanonicalSourceType(sourceType string) string {
	if canonical, ok := sourceTypeAliases[sourceType]; ok {
		return canonical
	}
	return sourceType
}

// identityKey is the canonical (tool, source type) pair of an entry.
type identityKey struct {
	tool       string
	sourceType string
}

func keyOf(e Entry) identityKey {
	return identityKey{tool: e.Tool, sourceType: CanonicalSourceType(GetSourceType(e.Source))}
}

func newKey(tool, sourceType string) identityKey {
	return identityKey{tool: tool, sourceType: CanonicalSourceType(sourceType)}
}

// readEntries reads the manifest file into an ordered slice of entries.
// Missing files are treated as empty (no entries), matching prior Get/Has behavior.
func readEntries(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}
		parts := strings.SplitN(line, ManifestDelimiter, ManifestFieldCount)
		if len(parts) == ManifestFieldCount {
			entries = append(entries, Entry{Tool: parts[0], Source: parts[1]})
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
func writeEntries(path string, entries []Entry) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".manifest-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	for _, e := range entries {
		if _, err := fmt.Fprintf(tmp, "%s%s%s\n", e.Tool, ManifestDelimiter, e.Source); err != nil {
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
// An entry is identified by (tool, source type): the matching line is
// updated in place (position preserved), a new pair is appended.
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

	want := newKey(tool, GetSourceType(source))
	found := false
	for i := range entries {
		if keyOf(entries[i]) == want {
			entries[i].Source = source
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, Entry{Tool: tool, Source: source})
	}

	return writeEntries(path, entries)
}

// AddMany applies several entry updates in a single read-modify-write, so N
// drift corrections cost one manifest rewrite instead of N. Per-entry
// semantics match Add: an existing (tool, source type) is updated in place
// (position preserved), a pair not yet tracked is appended. New pairs are
// appended sorted by tool then source type so the result is deterministic.
// An entry already recording the given source is left untouched and does not
// count toward the returned total; when nothing actually changes, no write
// is performed at all. Returns the number of entries changed.
func AddMany(updates []Entry) (int, error) {
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

	remaining := make(map[identityKey]string, len(updates))
	for _, u := range updates {
		remaining[keyOf(u)] = u.Source
	}

	changed := 0
	for i := range entries {
		k := keyOf(entries[i])
		source, ok := remaining[k]
		if !ok {
			continue
		}
		delete(remaining, k)
		if entries[i].Source == source {
			continue
		}
		entries[i].Source = source
		changed++
	}

	if len(remaining) > 0 {
		newKeys := make([]identityKey, 0, len(remaining))
		for k := range remaining {
			newKeys = append(newKeys, k)
		}
		sort.Slice(newKeys, func(i, j int) bool {
			if newKeys[i].tool != newKeys[j].tool {
				return newKeys[i].tool < newKeys[j].tool
			}
			return newKeys[i].sourceType < newKeys[j].sourceType
		})
		for _, k := range newKeys {
			entries = append(entries, Entry{Tool: k.tool, Source: remaining[k]})
			changed++
		}
	}

	if changed == 0 {
		return 0, nil
	}

	return changed, writeEntries(path, entries)
}

// Get retrieves the source string recorded for (tool, sourceType), or ""
// when that pair is not tracked.
func Get(tool, sourceType string) string {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	entries, err := readEntries(ManifestPath())
	if err != nil {
		return ""
	}
	want := newKey(tool, sourceType)
	for _, e := range entries {
		if keyOf(e) == want {
			return e.Source
		}
	}
	return ""
}

// Has checks if (tool, sourceType) exists in the manifest
func Has(tool, sourceType string) bool {
	return Get(tool, sourceType) != ""
}

// Lookup returns every entry recorded for a tool name, in file order. It is
// the only name-only lookup; everything else addresses a (tool, source type).
func Lookup(tool string) []Entry {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	entries, err := readEntries(ManifestPath())
	if err != nil {
		return nil
	}
	var found []Entry
	for _, e := range entries {
		if e.Tool == tool {
			found = append(found, e)
		}
	}
	return found
}

// All returns every entry in the system manifest, in file order
func All() ([]Entry, error) {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	return readEntries(ManifestPath())
}

// Remove removes the entry for (tool, sourceType), leaving other sources of
// the same tool name untouched.
func Remove(tool, sourceType string) error {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	entries, err := readEntries(path)
	if err != nil {
		return err
	}

	want := newKey(tool, sourceType)
	filtered := entries[:0]
	for _, e := range entries {
		if keyOf(e) != want {
			filtered = append(filtered, e)
		}
	}

	return writeEntries(path, filtered)
}

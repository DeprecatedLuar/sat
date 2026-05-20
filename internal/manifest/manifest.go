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

// Add adds a tool to the system manifest
// Format: tool=source:identity:version
func Add(tool, source string) error {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	if err := EnsureManifest(path); err != nil {
		return err
	}

	// Read existing entries
	entries := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}
		parts := strings.SplitN(line, ManifestDelimiter, ManifestFieldCount)
		if len(parts) == ManifestFieldCount {
			entries[parts[0]] = parts[1]
		}
	}
	file.Close()

	// Add/update entry
	entries[tool] = source

	// Write back
	return writeManifest(path, entries)
}

// Get retrieves the source string for a tool
func Get(tool string) string {
	manifestMutex.Lock()
	defer manifestMutex.Unlock()

	path := ManifestPath()
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ManifestDelimiter, ManifestFieldCount)
		if len(parts) == ManifestFieldCount && parts[0] == tool {
			return parts[1]
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
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	entries := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}
		parts := strings.SplitN(line, ManifestDelimiter, ManifestFieldCount)
		if len(parts) == ManifestFieldCount {
			entries[parts[0]] = parts[1]
		}
	}
	file.Close()

	delete(entries, tool)
	return writeManifest(path, entries)
}

// writeManifest writes entries to manifest file
func writeManifest(path string, entries map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for tool, source := range entries {
		fmt.Fprintf(f, "%s%s%s\n", tool, ManifestDelimiter, source)
	}
	return nil
}

package sources

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// FlatpakSearch searches Flathub
func FlatpakSearch(query string) ([]string, error) {
	// Check if flatpak is available
	if _, err := exec.LookPath("flatpak"); err != nil {
		return nil, nil
	}

	cmd := exec.Command("flatpak", "search", query)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, nil
	}

	// Parse flatpak search output (tab-separated: Name, Description, App ID, Version, Branch, Remotes)
	lines := strings.Split(output.String(), "\n")
	results := []string{}
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header line
		if strings.HasPrefix(line, "Name") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}

		// Extract fields
		// name := fields[0]
		desc := fields[1]
		appID := fields[2]
		version := fields[3]

		// Deduplicate by app ID
		if seen[appID] {
			continue
		}
		seen[appID] = true

		// Filter out plugins/extensions unless exact match
		queryLower := strings.ToLower(query)
		appIDLower := strings.ToLower(appID)
		if (strings.Contains(appIDLower, ".plugin.") ||
			strings.Contains(appIDLower, ".addon.") ||
			strings.Contains(appIDLower, ".extension.")) &&
			!strings.Contains(appIDLower, queryLower) {
			continue
		}

		result := fmt.Sprintf("%s %s - %s", appID, version, desc)
		results = append(results, result)

		// Limit to 5 results
		if len(results) >= 5 {
			break
		}
	}

	return results, nil
}

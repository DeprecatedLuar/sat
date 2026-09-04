package sources

// TODO(migration): this file only ports bash's inline PyPI search
// (lib/commands/search.sh's search_pypi, decoupled from any install source).
// The actual `uv` tool-manager adapter (install/uninstall/update/get-version,
// see lib/sources/uv.sh) has not been ported yet — there is no UvInstall/
// UvUninstall/etc. in this package.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// pypiPackageResponse represents PyPI package info API response
type pypiPackageResponse struct {
	Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Summary string `json:"summary"`
	} `json:"info"`
}

// PyPISearch searches PyPI for a package
func PyPISearch(query string) ([]string, error) {
	// PyPI doesn't have a search API endpoint anymore, we can only look up exact packages
	// Try to fetch the package info directly
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", query)
	data, err := common.FetchJSON(url, "pypi package")
	if err != nil {
		// Package not found or API error
		return nil, nil
	}

	var resp pypiPackageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil
	}

	desc := strings.Split(resp.Info.Summary, "\n")[0]
	if desc == "" {
		desc = "(no description)"
	}

	result := fmt.Sprintf("%s %s - %s", resp.Info.Name, resp.Info.Version, desc)
	return []string{result}, nil
}

// maxWheelBytes caps how large a wheel sat will download purely to read
// its console_scripts. Above this the cost outweighs the signal and
// binary provision is reported as unknown instead.
const maxWheelBytes = 12 << 20

// pypiLookupResponse is the PyPI package document, including the release
// files needed to inspect a wheel.
type pypiLookupResponse struct {
	Info struct {
		Name        string            `json:"name"`
		Version     string            `json:"version"`
		Summary     string            `json:"summary"`
		Yanked      bool              `json:"yanked"`
		HomePage    string            `json:"home_page"`
		ProjectURLs map[string]string `json:"project_urls"`
	} `json:"info"`
	URLs []struct {
		PackageType string `json:"packagetype"`
		URL         string `json:"url"`
		Size        int64  `json:"size"`
	} `json:"urls"`
}

// pypiRepoURLKeys are the project_urls entries that point at source, in
// the order they should be trusted. PyPI has no dedicated repository
// field, so the repo has to be recovered from this free-form map.
var pypiRepoURLKeys = []string{"Source", "Source Code", "Repository", "Code", "GitHub"}

// PyPILookup resolves an exact package name.
//
// PyPI's API does not expose entry points, so the only way to know whether
// a package installs an executable is to read entry_points.txt out of its
// wheel. That matters more here than elsewhere: PyPI is heavily
// name-squatted, and without this check every squatter survives as a
// plausible install candidate.
func PyPILookup(name string) (LookupResult, error) {
	data, err := common.FetchJSON(fmt.Sprintf("https://pypi.org/pypi/%s/json", name), "pypi lookup")
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var resp pypiLookupResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return LookupResult{}, ErrNoMatch
	}
	if !strings.EqualFold(resp.Info.Name, name) {
		return LookupResult{}, ErrNoMatch
	}

	result := LookupResult{
		Name:        resp.Info.Name,
		Version:     resp.Info.Version,
		Description: resp.Info.Summary,
		Homepage:    resp.Info.HomePage,
	}
	if resp.Info.Yanked {
		result.Dead, result.DeadReason = true, "yanked"
	}
	for _, key := range pypiRepoURLKeys {
		if u, ok := resp.Info.ProjectURLs[key]; ok && u != "" {
			result.Repo = u
			break
		}
	}

	// Prefer the smallest wheel: pure-python projects publish one, while
	// compiled projects publish many per-platform wheels whose entry
	// points are identical.
	var wheelURL string
	var wheelSize int64
	for _, u := range resp.URLs {
		if u.PackageType != "bdist_wheel" {
			continue
		}
		if wheelURL == "" || u.Size < wheelSize {
			wheelURL, wheelSize = u.URL, u.Size
		}
	}

	if wheelURL == "" || wheelSize > maxWheelBytes {
		return result, nil // BinsKnown stays false
	}
	bins, err := pypiWheelConsoleScripts(wheelURL)
	if err != nil {
		return result, nil
	}
	result.Bins, result.BinsKnown = bins, true
	return result, nil
}

// pypiWheelConsoleScripts downloads a wheel and returns the names declared
// under [console_scripts]. A wheel with no entry_points.txt installs no
// executables, which is exactly the signal wanted, so an absent file
// yields an empty list rather than an error.
func pypiWheelConsoleScripts(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWheelBytes))
	if err != nil {
		return nil, err
	}

	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, err
	}

	for _, f := range archive.File {
		if !strings.HasSuffix(f.Name, ".dist-info/entry_points.txt") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		return parseConsoleScripts(string(content)), nil
	}
	return nil, nil
}

// parseConsoleScripts extracts the keys of the [console_scripts] section
// of an INI-style entry_points.txt. Other sections (gui_scripts, plugin
// groups) are ignored because only console_scripts land on PATH.
func parseConsoleScripts(content string) []string {
	var names []string
	inSection := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSection = line == "[console_scripts]"
			continue
		}
		if !inSection {
			continue
		}
		if name, _, ok := strings.Cut(line, "="); ok {
			names = append(names, strings.TrimSpace(name))
		}
	}

	sort.Strings(names)
	return names
}

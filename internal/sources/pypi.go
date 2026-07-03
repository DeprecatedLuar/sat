package sources

// TODO(migration): this file only ports bash's inline PyPI search
// (lib/commands/search.sh's search_pypi, decoupled from any install source).
// The actual `uv` tool-manager adapter (install/uninstall/update/get-version,
// see lib/sources/uv.sh) has not been ported yet — there is no UvInstall/
// UvUninstall/etc. in this package.

import (
	"encoding/json"
	"fmt"
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

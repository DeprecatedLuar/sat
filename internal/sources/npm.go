package sources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// npmSearchResponse represents the npm registry search API response
type npmSearchResponse struct {
	Objects []struct {
		Package struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		} `json:"package"`
	} `json:"objects"`
}

// NpmSearch searches the npm registry
func NpmSearch(query string) ([]string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=%s&size=10", query)
	data, err := common.FetchJSON(url, "npm search")
	if err != nil {
		return nil, err
	}

	var resp npmSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	results := []string{}
	for _, obj := range resp.Objects {
		pkg := obj.Package
		desc := strings.Split(pkg.Description, "\n")[0]
		if desc == "" {
			desc = "(no description)"
		}
		result := fmt.Sprintf("%s %s - %s", pkg.Name, pkg.Version, desc)
		results = append(results, result)
	}

	return results, nil
}

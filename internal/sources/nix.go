package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// nixOSSearchURL is the NixOS package search backend used by search.nixos.org
// itself (its frontend queries this Elasticsearch/Bonsai cluster directly).
// The embedded credential is a fixed, publicly-known read-only key shared by
// every NixOS search client, not a project secret — bash's lib/sources/nix.sh
// uses the identical URL.
const nixOSSearchURL = "https://aWVSALXpZv:X8gPHnzL52wFEekuxsfQ9cSh@nixos-search-7-1733963800.us-east-1.bonsaisearch.net/latest-*/_search"

// NixOS search API response structures
// nixVersionRe extracts a version-shaped substring (e.g. "1.2.3") from
// nix-env output.
var nixVersionRe = regexp.MustCompile(`\d[\d.]+`)

// nixMaxSearchResults caps how many nix search results are returned.
const nixMaxSearchResults = 10

type nixSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				PackageAttrName    string `json:"package_attr_name"`
				PackagePVersion    string `json:"package_pversion"`
				PackageDescription string `json:"package_description"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// NixInstall installs a package via nix-env
func NixInstall(tool string) error {
	// Check if nix-env is available
	if _, err := exec.LookPath("nix-env"); err != nil {
		return fmt.Errorf("nix-env not installed")
	}

	// Skip on NixOS - packages should be managed declaratively
	if isNixOS() {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "[debug]   skipping nix-env on NixOS (use system config)\n")
		}
		return fmt.Errorf("nix packages should be managed declaratively on NixOS")
	}

	return common.RunQuiet("nix-env", "-iA", "nixpkgs."+tool)
}

// isNixOS checks if running on NixOS
func isNixOS() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "ID=nixos")
}

// NixUninstall removes a nix package
func NixUninstall(pkg, source string) error {
	// Try nix-env first, fall back to nix profile
	if err := common.RunQuiet("nix-env", "--uninstall", pkg); err != nil {
		return common.RunQuiet("nix", "profile", "remove", pkg)
	}
	return nil
}

// NixUpdate updates a nix package
func NixUpdate(tool string) error {
	return common.RunQuiet("nix-env", "-iA", "nixpkgs."+tool)
}

// NixGetVersion returns the installed version of a nix package
func NixGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("nix-env", "-q", tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Parse version from output
	if match := nixVersionRe.FindString(output.String()); match != "" {
		return match
	}

	return ""
}

// NixOSGetVersion returns the installed version of a NixOS system package
func NixOSGetVersion(tool string) string {
	binPath := filepath.Join("/run/current-system/sw/bin", tool)

	// Check if binary exists
	if _, err := os.Stat(binPath); err != nil {
		return ""
	}

	// Follow symlink to nix store
	storePath, err := filepath.EvalSymlinks(binPath)
	if err != nil {
		return ""
	}

	// Extract derivation name from store path
	// Example: /nix/store/hash-android-tools-35.0.2/bin/adb → android-tools-35.0.2
	parts := strings.Split(storePath, "/")
	if len(parts) < 4 {
		return ""
	}

	// Get store entry (e.g., "hash-android-tools-35.0.2")
	storeEntry := parts[3]

	// Extract derivation name (strip hash prefix)
	// hash-NAME-VERSION → NAME-VERSION
	dashIdx := strings.Index(storeEntry, "-")
	if dashIdx == -1 {
		return ""
	}
	deriv := storeEntry[dashIdx+1:]

	// Extract version (last dash-separated segment starting with digit)
	// android-tools-35.0.2 → 35.0.2
	re := regexp.MustCompile(`(^|-)(\d+\.[\d.]+\d+|[\d.]+)$`)
	if matches := re.FindStringSubmatch(deriv); len(matches) > 2 {
		return strings.TrimPrefix(matches[2], "-")
	}

	return ""
}

// NixQueryLatestVersion queries the latest version (not implemented)
func NixQueryLatestVersion(tool string) string {
	// Would need to query available packages
	var output bytes.Buffer
	cmd := exec.Command("nix-env", "-qaA", "nixpkgs."+tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Parse version from output
	if match := nixVersionRe.FindString(output.String()); match != "" {
		return match
	}

	return ""
}

// NixCheckOutdated checks if a nix package has updates available
func NixCheckOutdated(tool string, sourceType string) (current, latest string, err error) {
	// Skip nixos source - those are system packages managed declaratively
	if sourceType == "nixos" {
		return "", "", fmt.Errorf("nixos packages are managed declaratively")
	}

	// Check if nix-env is available
	if _, err := exec.LookPath("nix-env"); err != nil {
		return "", "", fmt.Errorf("nix-env not installed")
	}

	// Try manifest first, fall back to nix-env query
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}
	if current == "" {
		current = NixGetVersion(tool)
	}
	if current == "" {
		return "", "", fmt.Errorf("package not installed")
	}

	// Get latest from nix-env
	latest = NixQueryLatestVersion(tool)
	if latest == "" {
		return "", "", fmt.Errorf("failed to query latest version")
	}

	// Check if outdated
	if current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}

// NixSearch searches NixOS packages
func NixSearch(query string) ([]string, error) {
	url := nixOSSearchURL

	// Build search payload
	payload := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"package_attr_name": map[string]interface{}{
								"value": query,
								"boost": 10,
							},
						},
					},
					{
						"wildcard": map[string]interface{}{
							"package_attr_name": "*" + query + "*",
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
		"size": 20,
		"_source": []string{
			"package_attr_name",
			"package_pversion",
			"package_description",
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Execute POST request
	cmd := exec.Command("curl", "-sS", "--fail", "--max-time", "10",
		"-H", "Content-Type: application/json",
		"-d", string(payloadBytes),
		url)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		if os.Getenv(common.EnvSATDebug) != "" {
			fmt.Fprintf(os.Stderr, "[debug] failed to search NixOS packages\n")
		}
		return nil, fmt.Errorf("nix search API request failed")
	}

	// Parse response
	var resp nixSearchResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse nix search response")
	}

	// Deduplicate and format results
	seen := make(map[string]bool)
	results := []string{}

	for _, hit := range resp.Hits.Hits {
		src := hit.Source
		name := src.PackageAttrName

		// Skip duplicates
		if seen[name] {
			continue
		}
		seen[name] = true

		// Strip nix metadata from version
		version := src.PackagePVersion
		version = regexp.MustCompile(`-unstable-\d{4}-\d{2}-\d{2}`).ReplaceAllString(version, "")

		desc := src.PackageDescription
		if desc == "" {
			desc = "no description"
		}

		result := fmt.Sprintf("%s %s - %s", name, version, desc)
		results = append(results, result)

		// Limit to nixMaxSearchResults
		if len(results) >= nixMaxSearchResults {
			break
		}
	}

	return results, nil
}

// NixScan scans for installed nix user profile packages
func NixScan() ([]Package, error) {
	nixProfile := filepath.Join(os.Getenv("HOME"), ".nix-profile/bin")
	if !dirExists(nixProfile) {
		return nil, nil
	}

	var packages []Package
	entries, err := os.ReadDir(nixProfile)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil || !isExecutable(info) {
			continue
		}

		prog := entry.Name()
		version := NixGetVersion(prog)

		packages = append(packages, Package{
			Name:     prog,
			Source:   common.SourceNix,
			Identity: "",
			Version:  version,
		})
	}

	return packages, nil
}

// nixLookupResponse carries the nixpkgs fields an exact lookup needs.
// package_programs lists every executable the package installs and
// package_mainProgram names the primary one, which makes nix - alongside
// cargo and npm - one of the few sources that can prove binary provision.
type nixLookupResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				AttrName    string   `json:"package_attr_name"`
				PVersion    string   `json:"package_pversion"`
				Description string   `json:"package_description"`
				Programs    []string `json:"package_programs"`
				MainProgram string   `json:"package_mainProgram"`
				Homepage    []string `json:"package_homepage"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// nixLookupSize bounds how many hits the exact-name query returns. The
// backing URL uses a `latest-*` index wildcard, so one attribute can match
// in several channel indices at different versions; a handful of hits is
// enough to pick the newest.
const nixLookupSize = 20

// NixLookup resolves an exact nixpkgs attribute name.
//
// nixpkgs publishes no repository field - only package_homepage, which
// points at the project's website rather than its source - so Repo is left
// empty and callers resolve provenance from Homepage.
func NixLookup(name string) (LookupResult, error) {
	payload := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"type": "package"}},
					{"term": map[string]interface{}{"package_attr_name": name}},
				},
			},
		},
		"size": nixLookupSize,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return LookupResult{}, err
	}

	data, err := common.PostJSON(nixOSSearchURL, body, "nix lookup")
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var resp nixLookupResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return LookupResult{}, ErrNoMatch
	}
	if len(resp.Hits.Hits) == 0 {
		return LookupResult{}, ErrNoMatch
	}

	best := resp.Hits.Hits[0].Source
	for _, hit := range resp.Hits.Hits[1:] {
		if cmp, ok := common.CompareVersions(hit.Source.PVersion, best.PVersion); ok && cmp > 0 {
			best = hit.Source
		}
	}

	homepage := ""
	if len(best.Homepage) > 0 {
		homepage = best.Homepage[0]
	}

	// mainProgram names the executable users actually invoke; fall back to
	// the full program list when a package doesn't declare one.
	bins := best.Programs
	if best.MainProgram != "" {
		bins = []string{best.MainProgram}
	}

	return LookupResult{
		Name:        best.AttrName,
		Version:     best.PVersion,
		Description: best.Description,
		Homepage:    homepage,
		Bins:        bins,
		BinsKnown:   true,
	}, nil
}

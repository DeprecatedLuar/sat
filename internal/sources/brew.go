package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// Homebrew API response structures
type brewFormulaResponse struct {
	Name     string `json:"name"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Desc string `json:"desc"`
}

type brewCaskResponse struct {
	Token     string `json:"token"`
	Version   string `json:"version"`
	Desc      string `json:"desc"`
	DependsOn struct {
		MacOS interface{} `json:"macos"`
	} `json:"depends_on"`
}

type brewInfoResponse struct {
	Formulae []struct {
		Versions struct {
			Stable string `json:"stable"`
		} `json:"versions"`
	} `json:"formulae"`
	Casks []struct {
		Version string `json:"version"`
	} `json:"casks"`
}

// BrewInstall installs a package via Homebrew
func BrewInstall(tool string) error {
	// Check if brew is available
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("brew not installed")
	}

	// Try formula first
	cmd := exec.Command("brew", "info", tool)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if cmd.Run() == nil {
		// Formula exists, try to install
		var stderr bytes.Buffer
		installCmd := exec.Command("brew", "install", tool)
		installCmd.Stderr = &stderr

		if err := installCmd.Run(); err != nil {
			errOutput := stderr.String()
			if strings.Contains(errOutput, "macOS is required") {
				return fmt.Errorf("package requires macOS (brew cask)")
			}
			return fmt.Errorf("brew install failed: %s", errOutput)
		}
		return nil
	}

	// Try cask if formula not found
	cmd = exec.Command("brew", "info", "--cask", tool)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if cmd.Run() == nil {
		// Cask exists, try to install
		var stderr bytes.Buffer
		installCmd := exec.Command("brew", "install", "--cask", tool)
		installCmd.Stderr = &stderr

		if err := installCmd.Run(); err != nil {
			errOutput := stderr.String()
			if strings.Contains(errOutput, "macOS is required") {
				return fmt.Errorf("package requires macOS (brew cask)")
			}
			return fmt.Errorf("brew install --cask failed: %s", errOutput)
		}
		return nil
	}

	return fmt.Errorf("package not found in brew formulae or casks")
}

// BrewUninstall removes a Homebrew package
func BrewUninstall(pkg, source string) error {
	return common.RunQuiet("brew", "uninstall", pkg)
}

// BrewUpdate updates a Homebrew package
func BrewUpdate(tool string) error {
	return common.RunQuiet("brew", "upgrade", tool)
}

// BrewGetVersion returns the installed version of a brew package
func BrewGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("brew", "list", "--versions", tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Output: "tool version"
	fields := strings.Fields(strings.TrimSpace(output.String()))
	if len(fields) >= 2 {
		return fields[1]
	}

	return ""
}

// parseBrewListVersions parses `brew list --versions` (no argument = every
// installed formula/cask) output into a map keyed by name. A formula can
// list more than one installed version on one line (e.g. after `brew
// upgrade` without cleanup); the last one is what's actually linked/active.
func parseBrewListVersions(output string) map[string]string {
	versions := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 {
			versions[fields[0]] = fields[len(fields)-1]
		}
	}
	return versions
}

// BrewInstalledVersions resolves live versions for every given brew-tracked
// tool in a single `brew list --versions` call.
func BrewInstalledVersions(tools []string) map[string]string {
	if _, err := exec.LookPath("brew"); err != nil {
		return nil
	}
	var output bytes.Buffer
	cmd := exec.Command("brew", "list", "--versions")
	cmd.Stdout = &output
	cmd.Stderr = nil
	if cmd.Run() != nil {
		return nil
	}
	all := parseBrewListVersions(output.String())
	versions := make(map[string]string, len(tools))
	for _, tool := range tools {
		if v, ok := all[tool]; ok && v != "" {
			versions[tool] = v
		}
	}
	return versions
}

// BrewQueryLatestVersion queries the latest version from Homebrew API
func BrewQueryLatestVersion(tool string) string {
	// Try formula first
	url := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", tool)
	data, err := common.FetchJSON(url, fmt.Sprintf("brew/%s", tool))
	if err == nil {
		var resp brewFormulaResponse
		if json.Unmarshal(data, &resp) == nil && resp.Versions.Stable != "" {
			return resp.Versions.Stable
		}
	}

	// Try cask
	url = fmt.Sprintf("https://formulae.brew.sh/api/cask/%s.json", tool)
	data, err = common.FetchJSON(url, fmt.Sprintf("brew cask/%s", tool))
	if err == nil {
		var resp brewCaskResponse
		if json.Unmarshal(data, &resp) == nil && resp.Version != "" {
			return resp.Version
		}
	}

	return ""
}

// BrewCheckOutdated checks if a brew package has updates available
func BrewCheckOutdated(tool string) (current, latest string, err error) {
	// Check if brew is available
	if _, err := exec.LookPath("brew"); err != nil {
		return "", "", fmt.Errorf("brew not installed")
	}

	// Try manifest first
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}

	// Fall back to brew outdated if no manifest version
	if current == "" {
		var output bytes.Buffer
		cmd := exec.Command("brew", "outdated", "--verbose")
		cmd.Stdout = &output
		cmd.Stderr = nil

		if cmd.Run() != nil {
			return "", "", fmt.Errorf("brew outdated failed")
		}

		// Parse output: "tool (1.0.0) < 2.0.0"
		lines := strings.Split(output.String(), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, tool+" (") {
				// Extract version from parentheses
				start := strings.Index(line, "(")
				end := strings.Index(line, ")")
				if start != -1 && end != -1 && end > start {
					current = line[start+1 : end]
				}
				break
			}
		}
	}

	if current == "" {
		return "", "", fmt.Errorf("package not installed or not outdated")
	}

	// Get latest from brew info JSON
	var output bytes.Buffer
	cmd := exec.Command("brew", "info", "--json=v2", tool)
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return "", "", fmt.Errorf("brew info failed")
	}

	var info brewInfoResponse
	if err := json.Unmarshal(output.Bytes(), &info); err != nil {
		return "", "", fmt.Errorf("failed to parse brew info")
	}

	// Try formula first
	if len(info.Formulae) > 0 && info.Formulae[0].Versions.Stable != "" {
		latest = info.Formulae[0].Versions.Stable
	} else if len(info.Casks) > 0 && info.Casks[0].Version != "" {
		latest = info.Casks[0].Version
	}

	if latest == "" || current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}

// BrewSearch searches Homebrew formulae and casks
func BrewSearch(query string) ([]string, error) {
	results := []string{}

	// Try formula search
	url := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", query)
	data, err := common.FetchJSON(url, "brew formula")
	if err == nil {
		var resp brewFormulaResponse
		if json.Unmarshal(data, &resp) == nil && resp.Name != "" {
			desc := strings.Split(resp.Desc, "\n")[0]
			if desc == "" {
				desc = "(no description)"
			}
			result := fmt.Sprintf("%s %s - %s", resp.Name, resp.Versions.Stable, desc)
			results = append(results, result)
			return results, nil
		}
	}

	// Try cask search
	url = fmt.Sprintf("https://formulae.brew.sh/api/cask/%s.json", query)
	data, err = common.FetchJSON(url, "brew cask")
	if err == nil {
		var resp brewCaskResponse
		if json.Unmarshal(data, &resp) == nil && resp.Token != "" {
			desc := strings.Split(resp.Desc, "\n")[0]
			if desc == "" {
				desc = "(no description)"
			}
			// Check if macOS-only
			macOSOnly := ""
			if resp.DependsOn.MacOS != nil {
				macOSOnly = "(macOS only) "
			}
			result := fmt.Sprintf("%s %s - %s%s", resp.Token, resp.Version, macOSOnly, desc)
			results = append(results, result)
			return results, nil
		}
	}

	return nil, fmt.Errorf("package not found in brew formulae or casks")
}

// BrewScan scans for explicitly installed brew packages
func BrewScan() ([]Package, error) {
	if _, err := exec.LookPath("brew"); err != nil {
		return nil, nil
	}

	var packages []Package

	// Get brew leaves (explicit installs)
	var leavesOutput bytes.Buffer
	leavesCmd := exec.Command("brew", "leaves")
	leavesCmd.Stdout = &leavesOutput

	if leavesCmd.Run() != nil {
		return nil, nil
	}

	for _, formula := range strings.Split(leavesOutput.String(), "\n") {
		formula = strings.TrimSpace(formula)
		if formula == "" {
			continue
		}

		// Get actual binaries installed by this formula
		var listOutput bytes.Buffer
		listCmd := exec.Command("brew", "list", formula)
		listCmd.Stdout = &listOutput

		if listCmd.Run() != nil {
			continue
		}

		// Get version
		version := BrewGetVersion(formula)

		for _, line := range strings.Split(listOutput.String(), "\n") {
			if !strings.Contains(line, "/bin/") {
				continue
			}
			prog := strings.TrimSpace(line)
			prog = prog[strings.LastIndex(prog, "/")+1:]

			packages = append(packages, Package{
				Name:     prog,
				Source:   common.SourceBrew,
				Identity: "",
				Version:  version,
			})
		}
	}

	return packages, nil
}

// BrewManifestIssues reports brew-tracked manifest entries that are no
// longer explicitly installed - present only as a dependency of something
// else, per `brew leaves` - and should be pruned.
func BrewManifestIssues() ManifestIssues {
	var issues ManifestIssues

	leaves := brewLeavesSet()
	if len(leaves) == 0 {
		return issues
	}

	entries, err := manifest.All()
	if err != nil {
		return issues
	}

	for _, e := range entries {
		if manifest.GetSourceType(e.Source) != common.SourceBrew {
			continue
		}
		if !leaves[e.Tool] {
			issues.Prune = append(issues.Prune, PrunedEntry{Tool: e.Tool, Reason: "brew dep"})
		}
	}

	return issues
}

// brewLeavesSet returns the set of brew packages explicitly installed by
// the user (not pulled in only as a dependency of something else).
func brewLeavesSet() map[string]bool {
	leaves := make(map[string]bool)
	if _, err := exec.LookPath("brew"); err != nil {
		return leaves
	}

	var output bytes.Buffer
	cmd := exec.Command("brew", "leaves")
	cmd.Stdout = &output
	if cmd.Run() == nil {
		for _, leaf := range strings.Split(output.String(), "\n") {
			leaf = strings.TrimSpace(leaf)
			if leaf != "" {
				leaves[leaf] = true
			}
		}
	}
	return leaves
}

// brewLookupResponse is the formulae.brew.sh document for one formula,
// carrying the provenance and lifecycle fields the exact lookup needs but
// brewFormulaResponse (used by search) does not.
type brewLookupResponse struct {
	Name       string `json:"name"`
	FullName   string `json:"full_name"`
	Desc       string `json:"desc"`
	Homepage   string `json:"homepage"`
	Deprecated bool   `json:"deprecated"`
	Disabled   bool   `json:"disabled"`
	Versions   struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	URLs struct {
		Stable struct {
			URL string `json:"url"`
		} `json:"stable"`
	} `json:"urls"`
}

// BrewLookup resolves an exact formula name.
//
// The stable source URL identifies the upstream far better than homepage
// does - brew's jq builds from github.com/jqlang/jq while its homepage is
// jqlang.github.io - so it is preferred when it points at a repository.
// Formulae built from a language registry (prettier from npm, yt-dlp from
// PyPI) are resolved back to that package's own repo, otherwise they look
// like a separate project.
//
// The API exposes no provided-binary list, so BinsKnown stays false.
func BrewLookup(name string) (LookupResult, error) {
	data, err := common.FetchJSON("https://formulae.brew.sh/api/formula/"+name+".json", "brew lookup")
	if err != nil {
		return LookupResult{}, ErrNoMatch
	}

	var formula brewLookupResponse
	if err := json.Unmarshal(data, &formula); err != nil {
		return LookupResult{}, ErrNoMatch
	}
	if formula.Name != name {
		return LookupResult{}, ErrNoMatch
	}

	result := LookupResult{
		Name:        formula.FullName,
		Version:     formula.Versions.Stable,
		Description: formula.Desc,
		Homepage:    formula.Homepage,
		Repo:        brewUpstreamRepo(formula.URLs.Stable.URL),
	}
	switch {
	case formula.Disabled:
		result.Dead, result.DeadReason = true, "disabled"
	case formula.Deprecated:
		result.Dead, result.DeadReason = true, "deprecated"
	}
	return result, nil
}

// brewUpstreamRepo turns a formula's stable source URL into a repository
// URL when it is one, following language-registry tarballs back to the
// package that published them.
func brewUpstreamRepo(stableURL string) string {
	if stableURL == "" {
		return ""
	}
	if IsRepoURL(CanonicalRepo(stableURL)) {
		return stableURL
	}
	if repo := registryTarballRepo(stableURL); repo != "" {
		return repo
	}
	return ""
}

// registryTarballRepo resolves a tarball URL served by npm or PyPI back to
// the repository of the package it belongs to. Formulae that repackage a
// language-registry release would otherwise present the tarball host as
// their upstream and never group with the same project from elsewhere.
func registryTarballRepo(tarballURL string) string {
	if name, ok := npmTarballPackage(tarballURL); ok {
		if res, err := NpmLookup(name); err == nil && res.Repo != "" {
			return res.Repo
		}
		return ""
	}
	if name, ok := pypiTarballPackage(tarballURL); ok {
		if res, err := PyPILookup(name); err == nil && res.Repo != "" {
			return res.Repo
		}
	}
	return ""
}

// npmTarballPackage extracts the package name from a registry tarball URL
// such as https://registry.npmjs.org/prettier/-/prettier-3.9.6.tgz.
func npmTarballPackage(u string) (string, bool) {
	rest, ok := strings.CutPrefix(CanonicalRepo(u), "registry.npmjs.org/")
	if !ok {
		return "", false
	}
	name, _, found := strings.Cut(rest, "/-/")
	if !found || name == "" {
		return "", false
	}
	return name, true
}

// pypiTarballPackage extracts the distribution name from a PyPI file URL
// such as https://files.pythonhosted.org/packages/../yt_dlp-2026.7.4.tar.gz.
// PyPI normalizes underscores to hyphens in project names.
func pypiTarballPackage(u string) (string, bool) {
	if !strings.Contains(u, "files.pythonhosted.org/") {
		return "", false
	}
	base := u[strings.LastIndex(u, "/")+1:]
	name, _, found := strings.Cut(base, "-")
	if !found || name == "" {
		return "", false
	}
	return strings.ReplaceAll(name, "_", "-"), true
}

package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const (
	// Search result limits
	MaxSearchResults = 30

	// Package manager names (used for routing)
	PkgMgrAPT     = "apt"
	PkgMgrPacman  = "pacman"
	PkgMgrAPK     = "apk"
	PkgMgrDNF     = "dnf"
	PkgMgrNix     = "nix"
	PkgMgrUnknown = "unknown"
)

// Install installs a package via the system package manager
func Install(tool string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanInstall(tool)
	case PkgMgrAPT:
		return aptInstall(tool)
	case PkgMgrDNF:
		return dnfInstall(tool)
	case PkgMgrAPK:
		return apkInstall(tool)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// Uninstall removes a system package
func Uninstall(pkg, source string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanUninstall(pkg)
	case PkgMgrAPT:
		return aptUninstall(pkg)
	case PkgMgrDNF:
		return dnfUninstall(pkg)
	case PkgMgrAPK:
		return apkUninstall(pkg)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// Update updates a system package
func Update(tool string) error {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return fmt.Errorf("no system package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanUpdate(tool)
	case PkgMgrAPT:
		return aptUpdate(tool)
	case PkgMgrDNF:
		return dnfUpdate(tool)
	case PkgMgrAPK:
		return apkUpdate(tool)
	default:
		return fmt.Errorf("unsupported package manager: %s", mgr)
	}
}

// GetVersion returns the installed version of a system package
func GetVersion(tool string) string {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return ""
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanGetVersion(tool)
	case PkgMgrAPT:
		return aptGetVersion(tool)
	case PkgMgrDNF:
		return dnfGetVersion(tool)
	case PkgMgrAPK:
		return apkGetVersion(tool)
	default:
		return ""
	}
}

// InstalledVersions resolves live versions for every given system-package
// tool in one bulk call to whichever package manager is detected, mirroring
// GetVersion's per-manager dispatch.
func InstalledVersions(tools []string) map[string]string {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return nil
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanInstalledVersions(tools)
	case PkgMgrAPT:
		return aptInstalledVersions(tools)
	case PkgMgrDNF:
		return dnfInstalledVersions(tools)
	case PkgMgrAPK:
		return apkInstalledVersions(tools)
	default:
		return nil
	}
}

// QueryLatestVersion queries the latest available version (not implemented for system packages)
// System packages don't have a simple "latest version" query without updating repos
func QueryLatestVersion(tool string) string {
	return "" // Not implemented for system packages
}

// CheckOutdated checks if a system package has updates available
// Returns current version, latest version, and error
func CheckOutdated(tool string) (current, latest string, err error) {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return "", "", fmt.Errorf("no package manager detected")
	}

	// Only apt has reliable per-package outdated checks without sudo
	if mgr == PkgMgrAPT {
		return aptCheckOutdated(tool)
	}

	// For other package managers, return error (no reliable check without sudo)
	return "", "", fmt.Errorf("outdated check not supported for %s", mgr)
}

// Search searches for packages in the system repository
func Search(query string) ([]string, error) {
	mgr := common.GetPkgManager()
	if mgr == "" {
		return nil, fmt.Errorf("no package manager detected")
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanSearch(query)
	case PkgMgrAPT:
		return aptSearch(query)
	case PkgMgrDNF:
		return dnfSearch(query)
	case PkgMgrAPK:
		return apkSearch(query)
	default:
		return nil, fmt.Errorf("search not supported for %s", mgr)
	}
}

// SystemScan scans for explicitly-installed system packages
func SystemScan() ([]Package, error) {
	mgr := common.GetPkgManager()
	if mgr == "" || mgr == PkgMgrUnknown {
		return nil, nil
	}

	switch mgr {
	case PkgMgrPacman:
		return pacmanScan()
	case PkgMgrAPT:
		return aptScan()
	case PkgMgrDNF:
		return dnfScan()
	case PkgMgrAPK:
		return apkScan()
	case PkgMgrNix:
		// NixOS system packages (declarative, read-only)
		return nixOSScan()
	default:
		return nil, nil
	}
}

// Debug helper
func init() {
	if os.Getenv(common.EnvSATDebug) != "" {
		fmt.Fprintf(os.Stderr, "%s system source module loaded\n", common.DebugPrefix)
	}
}

// System package indices queried by SystemLookup. `system` is an
// abstraction over whichever native package manager the running distro
// uses, so these are alternatives describing one source, not competitors.
const (
	archPackageAPI   = "https://archlinux.org/packages/search/json/?name="
	debianSourceAPI  = "https://sources.debian.org/api/src/"
	fedoraPackageAPI = "https://mdapi.fedoraproject.org/f43/pkg/"
	ubuntuPackageAPI = "https://api.launchpad.net/1.0/ubuntu/+archive/primary" +
		"?ws.op=getPublishedSources&exact_match=true&status=Published&source_name="
)

// distroLookup queries one distribution's package index.
type distroLookup struct {
	name  string
	query func(string) (version, repo, description string, err error)
}

// systemDistros are probed together because sat cannot know which distro a
// manifest entry will be installed on, and only one is ever reachable on a
// given machine.
var systemDistros = []distroLookup{
	{"arch", archLookup},
	{"debian", debianLookup},
	{"fedora", fedoraLookup},
	{"ubuntu", ubuntuLookup},
}

// SystemLookup resolves an exact package name across the distro indices
// and collapses them into a single result, reporting the newest version
// found. The distros that carried the package are listed in Name so the
// caller can show which ones agreed.
//
// Distro indices publish no provided-binary list, so BinsKnown stays
// false.
func SystemLookup(name string) (LookupResult, error) {
	type hit struct{ distro, version, repo, description string }

	var mu sync.Mutex
	var wg sync.WaitGroup
	var hits []hit

	for _, distro := range systemDistros {
		wg.Add(1)
		go func(d distroLookup) {
			defer wg.Done()
			version, repo, description, err := d.query(name)
			if err != nil || version == "" {
				return
			}
			mu.Lock()
			hits = append(hits, hit{d.name, version, repo, description})
			mu.Unlock()
		}(distro)
	}
	wg.Wait()

	if len(hits) == 0 {
		return LookupResult{}, ErrNoMatch
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].distro < hits[j].distro })

	best := 0
	labels := make([]string, 0, len(hits))
	result := LookupResult{}
	for i, h := range hits {
		labels = append(labels, h.distro+" "+h.version)
		// Fail-closed: an undecidable comparison leaves the incumbent in
		// place. Distro version strings are noisy enough that a fail-open
		// comparison would let the last-listed distro always "win".
		if i > 0 {
			if cmp, ok := common.CompareVersions(h.version, hits[best].version); ok && cmp > 0 {
				best = i
			}
		}
		if result.Repo == "" {
			result.Repo = h.repo
		}
		if result.Description == "" {
			result.Description = h.description
		}
	}

	result.Name = name + " [" + strings.Join(labels, ", ") + "]"
	result.Version = hits[best].version
	return result, nil
}

func archLookup(name string) (string, string, string, error) {
	data, err := common.FetchJSON(archPackageAPI+name, "arch lookup")
	if err != nil {
		return "", "", "", err
	}

	var resp struct {
		Results []struct {
			PkgName string `json:"pkgname"`
			PkgVer  string `json:"pkgver"`
			URL     string `json:"url"`
			PkgDesc string `json:"pkgdesc"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", "", err
	}

	for _, r := range resp.Results {
		if r.PkgName == name {
			return r.PkgVer, r.URL, r.PkgDesc, nil
		}
	}
	return "", "", "", ErrNoMatch
}

// debianLookup reports the newest version across Debian suites. Stable
// deliberately lags, so taking the newest keeps `system` from looking
// stale purely because an old suite is still published.
func debianLookup(name string) (string, string, string, error) {
	data, err := common.FetchJSON(debianSourceAPI+name+"/", "debian lookup")
	if err != nil {
		return "", "", "", err
	}

	var resp struct {
		Package  string `json:"package"`
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", "", err
	}
	if resp.Package != name || len(resp.Versions) == 0 {
		return "", "", "", ErrNoMatch
	}

	best := resp.Versions[0].Version
	for _, v := range resp.Versions[1:] {
		if cmp, ok := common.CompareVersions(v.Version, best); ok && cmp > 0 {
			best = v.Version
		}
	}
	return best, "", "", nil
}

func fedoraLookup(name string) (string, string, string, error) {
	data, err := common.FetchJSON(fedoraPackageAPI+name, "fedora lookup")
	if err != nil {
		return "", "", "", err
	}

	var resp struct {
		Basename string `json:"basename"`
		Version  string `json:"version"`
		URL      string `json:"url"`
		Summary  string `json:"summary"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", "", err
	}
	if resp.Basename != name {
		return "", "", "", ErrNoMatch
	}
	return resp.Version, resp.URL, resp.Summary, nil
}

func ubuntuLookup(name string) (string, string, string, error) {
	data, err := common.FetchJSON(ubuntuPackageAPI+name, "ubuntu lookup")
	if err != nil {
		return "", "", "", err
	}

	var resp struct {
		Entries []struct {
			Version string `json:"source_package_version"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", "", err
	}
	if len(resp.Entries) == 0 {
		return "", "", "", ErrNoMatch
	}

	best := resp.Entries[0].Version
	for _, e := range resp.Entries[1:] {
		if cmp, ok := common.CompareVersions(e.Version, best); ok && cmp > 0 {
			best = e.Version
		}
	}
	return best, "", "", nil
}

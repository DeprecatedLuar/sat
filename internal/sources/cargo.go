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

// Package represents a discovered package
type Package struct {
	Name     string
	Source   string
	Identity string
	Version  string
}

// Cargo API response structures
type cargoSearchResponse struct {
	Crates []struct {
		Name        string `json:"name"`
		MaxVersion  string `json:"max_version"`
		Description string `json:"description"`
	} `json:"crates"`
}

type cargoVersionResponse struct {
	Crate struct {
		NewestVersion string `json:"newest_version"`
	} `json:"crate"`
}

// CargoInstall installs a crate via cargo
func CargoInstall(tool string) error {
	// Check if cargo is available
	if _, err := exec.LookPath("cargo"); err != nil {
		return fmt.Errorf("cargo not installed")
	}

	// Try install
	if os.Getenv(common.EnvSATDebug) != "" {
		cmd := exec.Command("cargo", "install", tool)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Capture errors to detect missing build dependencies
	var stderr bytes.Buffer
	cmd := exec.Command("cargo", "install", tool)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		return nil
	}

	// Check for missing build tools
	errOutput := stderr.String()
	re := regexp.MustCompile("is `([^`]+)` not installed")
	matches := re.FindStringSubmatch(errOutput)

	if len(matches) > 1 {
		missing := matches[1]
		fmt.Printf("\r%-50s\r", "")
		fmt.Printf("\033[2mBuild requires %s, installing...\033[0m\n", missing)

		// Try brew first (no sudo)
		if _, err := exec.LookPath("brew"); err == nil {
			if common.RunQuiet("brew", "install", missing) == nil {
				fmt.Printf("[\033[0;92m✓\033[0m] %-20s [\033[38;2;255;200;100mbrew\033[0m] \033[2m(build dep)\033[0m\n", missing)
				if common.RunQuiet("cargo", "install", tool) == nil {
					return nil
				}
			}
		}

		// Try system package manager
		mgr := common.GetPkgManager()
		if mgr != "" && mgr != PkgMgrUnknown {
			if common.PkgInstall(missing, mgr) == nil {
				fmt.Printf("[\033[0;92m✓\033[0m] %-20s [\033[38;2;120;200;255msystem\033[0m] \033[2m(build dep)\033[0m\n", missing)
				if common.RunQuiet("cargo", "install", tool) == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("cargo install failed")
}

// CargoUninstall removes a cargo package
func CargoUninstall(pkg, source string) error {
	// Binary name may differ from crate name - look it up first
	crate := getCrateName(pkg)
	if crate == "" {
		crate = pkg
	}

	return common.RunQuiet("cargo", "uninstall", crate)
}

// getCrateName finds the crate name for a given binary
func getCrateName(binary string) string {
	var output bytes.Buffer
	cmd := exec.Command("cargo", "install", "--list")
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Parse output to find crate name
	// Format:
	//   crate-name v1.0.0:
	//       binary1
	//       binary2
	lines := strings.Split(output.String(), "\n")
	var currentCrate string

	for _, line := range lines {
		// Crate lines don't start with whitespace
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			// Parse: "crate-name v1.0.0:"
			fields := strings.Fields(line)
			if len(fields) > 0 {
				currentCrate = fields[0]
			}
		} else if strings.TrimSpace(line) == binary && currentCrate != "" {
			// Found matching binary under current crate
			return currentCrate
		}
	}

	return ""
}

// CargoUpdate updates a cargo package
func CargoUpdate(tool string) error {
	// cargo install re-installs/updates
	return common.RunQuiet("cargo", "install", tool)
}

// CargoGetVersion returns the installed version of a cargo package
func CargoGetVersion(tool string) string {
	var output bytes.Buffer
	cmd := exec.Command("cargo", "install", "--list")
	cmd.Stdout = &output
	cmd.Stderr = nil

	if cmd.Run() != nil {
		return ""
	}

	// Parse: "tool v1.0.0:"
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(tool) + ` v([^ :]+)`)
	lines := strings.Split(output.String(), "\n")
	for _, line := range lines {
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// CargoQueryLatestVersion queries the latest version from crates.io
func CargoQueryLatestVersion(tool string) string {
	url := fmt.Sprintf("https://crates.io/api/v1/crates/%s", tool)
	data, err := common.FetchJSON(url, fmt.Sprintf("crates.io/%s", tool))
	if err != nil {
		return ""
	}

	var resp cargoVersionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}

	return resp.Crate.NewestVersion
}

// CargoCheckOutdated checks if a cargo package has updates available
func CargoCheckOutdated(tool string) (current, latest string, err error) {
	// Check if cargo is available
	if _, err := exec.LookPath("cargo"); err != nil {
		return "", "", fmt.Errorf("cargo not installed")
	}

	// Try manifest first, fall back to querying cargo
	if sourceStr := manifest.Get(tool); sourceStr != "" {
		current = manifest.GetSourceVersion(sourceStr)
	}
	if current == "" {
		current = CargoGetVersion(tool)
	}
	if current == "" {
		return "", "", fmt.Errorf("package not installed")
	}

	// Get latest from crates.io
	latest = CargoQueryLatestVersion(tool)
	if latest == "" {
		return "", "", fmt.Errorf("failed to query crates.io")
	}

	// Check if outdated
	if current == latest {
		return "", "", fmt.Errorf("already up to date")
	}

	return current, latest, nil
}

// CargoSearch searches crates.io
func CargoSearch(query string) ([]string, error) {
	url := fmt.Sprintf("https://crates.io/api/v1/crates?q=%s&per_page=10", query)
	data, err := common.FetchJSON(url, "crates.io search")
	if err != nil {
		return nil, err
	}

	var resp cargoSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	results := []string{}
	for _, crate := range resp.Crates {
		// Get first line of description
		desc := strings.Split(crate.Description, "\n")[0]
		if desc == "" {
			desc = "(no description)"
		}
		result := fmt.Sprintf("%s %s - %s", crate.Name, crate.MaxVersion, desc)
		results = append(results, result)
	}

	return results, nil
}

// CargoScan scans for installed cargo packages
func CargoScan() ([]Package, error) {
	dir := cargoBinDir()
	if dir == "" || !dirExists(dir) {
		return nil, nil
	}

	var packages []Package
	entries, err := os.ReadDir(dir)
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
		version := CargoGetVersion(prog)

		packages = append(packages, Package{
			Name:     prog,
			Source:   common.SourceCargo,
			Identity: "",
			Version:  version,
		})
	}

	return packages, nil
}

// Helper functions

func cargoBinDir() string {
	cargoHome := os.Getenv("CARGO_HOME")
	if cargoHome == "" {
		cargoHome = filepath.Join(os.Getenv("HOME"), ".cargo")
	}
	return filepath.Join(cargoHome, "bin")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isExecutable(info os.FileInfo) bool {
	return info.Mode()&0111 != 0
}

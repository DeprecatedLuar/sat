// Package config manages sat's user-editable configuration file
// (~/.config/sat/config.toml), currently limited to the install
// fallback-chain order.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// Environment variables
	EnvXDGConfigHome = "XDG_CONFIG_HOME"
	EnvHome          = "HOME"

	// Paths
	DefaultConfigDir = ".config"
	SatConfigDir     = "sat"
	ConfigFileName   = "config.toml"

	// File permissions
	FilePermissions = 0644
	DirPermissions  = 0755
)

// DefaultInstallOrder is the fallback chain used for `sat install` when no
// config.toml exists yet, and the contents written into the pregenerated
// default file. Matches bash's documented INSTALL_ORDER
// (lib/commands/install.sh).
var DefaultInstallOrder = []string{"brew", "nix", "system", "cargo", "uv", "npm", "sat", "gh"}

// defaultConfigTemplate is written verbatim on first run so the file is
// self-documenting; Load() parses whatever the user leaves behind.
const defaultConfigTemplate = `# sat configuration
#
# order: the fallback chain tried by "sat install <tool>" when no source is
# forced (no ":source" suffix, no --flag). Sources are tried left to right;
# the first one that has the package wins. Sources omitted from this array
# are never tried at all.
order = ["brew", "nix", "system", "cargo", "uv", "npm", "sat", "gh"]
`

// Config is the parsed contents of config.toml.
type Config struct {
	InstallOrder []string `toml:"order"`
}

// dir returns ~/.config/sat (respecting XDG_CONFIG_HOME).
func dir() string {
	configHome := os.Getenv(EnvXDGConfigHome)
	if configHome == "" {
		configHome = filepath.Join(os.Getenv(EnvHome), DefaultConfigDir)
	}
	return filepath.Join(configHome, SatConfigDir)
}

// path returns ~/.config/sat/config.toml (respecting XDG_CONFIG_HOME).
func path() string {
	return filepath.Join(dir(), ConfigFileName)
}

// EnsureDefault writes the default config.toml if it doesn't exist yet.
// Idempotent - safe to call on every invocation.
func EnsureDefault() error {
	p := path()
	if _, err := os.Stat(p); err == nil {
		return nil
	}

	if err := os.MkdirAll(dir(), DirPermissions); err != nil {
		return err
	}

	return os.WriteFile(p, []byte(defaultConfigTemplate), FilePermissions)
}

// Load reads config.toml, falling back to DefaultInstallOrder for any
// field left unset (including when the file doesn't exist at all).
func Load() (Config, error) {
	cfg := Config{InstallOrder: DefaultInstallOrder}

	data, err := os.ReadFile(path())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var parsed Config
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return cfg, err
	}

	if parsed.InstallOrder != nil {
		cfg.InstallOrder = parsed.InstallOrder
	}

	return cfg, nil
}

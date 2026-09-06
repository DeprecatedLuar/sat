// Package config manages sat's user-editable configuration file
// (~/.config/sat/config.toml), currently limited to the install
// fallback-chain order.
package config

import (
	"os"
	"path/filepath"
	"time"

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

// DefaultDriftInterval is how long a version-drift reconciliation is
// considered fresh: short enough that a tool updated outside sat (or one
// that self-updates) self-corrects the same day, long enough that a burst
// of sat invocations pays the reconcile cost once.
const DefaultDriftInterval = 12 * time.Hour

// defaultConfigTemplate is written verbatim on first run so the file is
// self-documenting; Load() parses whatever the user leaves behind.
const defaultConfigTemplate = `# sat configuration
#
# order: the fallback chain tried by "sat install <tool>" when no source is
# forced (no ":source" suffix, no --flag). Sources are tried left to right;
# the first one that has the package wins. Sources omitted from this array
# are never tried at all.
order = ["brew", "nix", "system", "cargo", "uv", "npm", "sat", "gh"]

# drift_interval: how often sat re-checks installed versions against what
# the manifest recorded, so tools updated outside sat (or that self-update)
# stop showing a stale version. Any Go duration ("6h", "30m"); "off"
# disables it. sat update and sat scan always reconcile regardless.
drift_interval = "12h"
`

// Config is the parsed contents of config.toml.
type Config struct {
	InstallOrder  []string `toml:"order"`
	DriftInterval string   `toml:"drift_interval"`
}

// ResolvedDriftInterval parses DriftInterval into a duration; 0 means
// disabled. An unparseable value (a typo in config.toml) falls back to
// DefaultDriftInterval rather than erroring - a bad edit must not break
// every sat invocation, since this is checked on every one of them.
func (c Config) ResolvedDriftInterval() time.Duration {
	switch c.DriftInterval {
	case "off", "never", "0":
		return 0
	}
	d, err := time.ParseDuration(c.DriftInterval)
	if err != nil {
		return DefaultDriftInterval
	}
	return d
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

// Load reads config.toml, falling back to DefaultInstallOrder/
// DefaultDriftInterval for any field left unset (including when the file
// doesn't exist at all, or predates a field - EnsureDefault never rewrites
// an existing file, so an upgraded sat must tolerate older configs missing
// newer keys).
func Load() (Config, error) {
	cfg := Config{InstallOrder: DefaultInstallOrder, DriftInterval: DefaultDriftInterval.String()}

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
	if parsed.DriftInterval != "" {
		cfg.DriftInterval = parsed.DriftInterval
	}

	return cfg, nil
}

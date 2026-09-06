package config

import (
	"testing"
	"time"
)

func TestResolvedDriftInterval(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"empty falls back to default", "", DefaultDriftInterval},
		{"garbage falls back to default", "not-a-duration", DefaultDriftInterval},
		{"explicit duration", "30m", 30 * time.Minute},
		{"hours", "6h", 6 * time.Hour},
		{"off disables", "off", 0},
		{"never disables", "never", 0},
		{"zero disables", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{DriftInterval: tt.value}
			if got := cfg.ResolvedDriftInterval(); got != tt.want {
				t.Errorf("ResolvedDriftInterval(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestLoadDriftIntervalFallback(t *testing.T) {
	// A Config zero value (as if the key were entirely absent from an old
	// config.toml) must resolve to the documented default, not to 0/disabled -
	// Load() is responsible for filling DriftInterval before a caller ever
	// sees the struct; this locks in what an absent key must resolve to.
	cfg := Config{}
	if got := cfg.ResolvedDriftInterval(); got != DefaultDriftInterval {
		t.Errorf("zero-value Config.ResolvedDriftInterval() = %v, want %v (an absent key must not silently disable)", got, DefaultDriftInterval)
	}
}

package sources

import (
	"testing"
)

func TestCargoQueryLatestVersion(t *testing.T) {
	version := CargoQueryLatestVersion("ripgrep")
	if version == "" {
		t.Errorf("CargoQueryLatestVersion(\"ripgrep\") returned empty string, expected non-empty version")
	}
	t.Logf("ripgrep latest version: %s", version)
}

package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeprecatedLuar/sat/internal/common"
)

// setupSourceBinEnv isolates HOME, SAT_DATA and PATH in temp dirs.
func setupSourceBinEnv(t *testing.T) (nixDir string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("SAT_DATA", filepath.Join(root, "data"))
	nixDir = filepath.Join(root, "nix")
	for _, d := range []string{common.LocalBin(), nixDir, common.AppImagesDir(), common.FlatpakWrapperDir(), common.HuberBinDir()} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{common.LocalBin(), nixDir}, string(os.PathListSeparator)))
	return nixDir
}

func TestFlatpakWrapperNamedByToolNameAndStableAfterReconcile(t *testing.T) {
	setupSourceBinEnv(t)
	const appID = "org.telegram.desktop"
	name := FlatpakToolName(appID, map[string]string{appID: "Telegram"})
	if name != "telegram" {
		t.Fatalf("tool name %q, want telegram", name)
	}

	if err := EnsureFlatpakWrapper(appID, name); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(common.FlatpakWrapperDir(), "telegram")) {
		t.Fatal("wrapper not named telegram")
	}
	if reconcileWrapper(appID, name) {
		t.Fatal("reconcile recreated an existing wrapper")
	}
	entries, err := os.ReadDir(common.FlatpakWrapperDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "telegram" {
		t.Fatalf("wrapper dir changed after reconcile: %v", entries)
	}
}

func TestFlatpakWrapperNameClashErrors(t *testing.T) {
	setupSourceBinEnv(t)
	if err := EnsureFlatpakWrapper("org.a.App", "app"); err != nil {
		t.Fatal(err)
	}
	err := EnsureFlatpakWrapper("org.b.App", "app")
	if err == nil || !strings.Contains(err.Error(), "org.a.App") || !strings.Contains(err.Error(), "org.b.App") {
		t.Fatalf("expected error naming both app IDs, got %v", err)
	}
}

func TestGitHubUninstallFallbackRemovesNaturalHuberBinary(t *testing.T) {
	nixDir := setupSourceBinEnv(t)
	if err := os.WriteFile(filepath.Join(nixDir, "foo"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	huberBin := filepath.Join(common.HuberBinDir(), "foo")
	if err := os.WriteFile(huberBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	name, err := common.LinkBin(huberBin)
	if err != nil || name != "foo-gh" {
		t.Fatalf("got %q, %v; want foo-gh", name, err)
	}

	if err := GitHubUninstall("foo", ""); err != nil {
		t.Fatal(err)
	}
	if fileExists(huberBin) {
		t.Fatal("huber binary left behind")
	}
	if _, err := os.Lstat(filepath.Join(common.LocalBin(), "foo-gh")); !os.IsNotExist(err) {
		t.Fatal("suffixed link left behind")
	}
}

func TestReconcileWrapperRenamesIdDerivedWrapper(t *testing.T) {
	setupSourceBinEnv(t)
	const appID = "org.telegram.desktop"
	if err := createFlatpakWrapper("desktop", appID); err != nil {
		t.Fatal(err)
	}

	if reconcileWrapper(appID, "telegram") {
		t.Fatal("rename reported as create")
	}
	common.ReconcileBinLinks()

	entries, err := os.ReadDir(common.FlatpakWrapperDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "telegram" {
		t.Fatalf("expected one wrapper named telegram, got %v", entries)
	}
	if _, err := os.Lstat(filepath.Join(common.LocalBin(), "desktop")); !os.IsNotExist(err) {
		t.Fatal("link to the old wrapper name left behind")
	}
	if _, err := os.Lstat(filepath.Join(common.LocalBin(), "telegram")); err != nil {
		t.Fatal("link for the renamed wrapper missing")
	}
}

func TestRepointDesktopExecRewritesLinkPathOnce(t *testing.T) {
	setupSourceBinEnv(t)
	if err := os.MkdirAll(common.AppImageApplicationsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	stored := filepath.Join(common.AppImagesDir(), "jackify")
	entry := filepath.Join(common.AppImageApplicationsDir(), "jackify.desktop")
	content := "[Desktop Entry]\nName=Jackify\nExec=" + filepath.Join(common.LocalBin(), "jackify") + " %U\nIcon=x\n"
	if err := os.WriteFile(entry, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := repointDesktopExec("jackify", stored); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	want := "[Desktop Entry]\nName=Jackify\nExec=" + stored + " %U\nIcon=x\n"
	if string(data) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", data, want)
	}

	if err := repointDesktopExec("jackify", stored); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(entry)
	if string(again) != want {
		t.Fatal("second run changed the entry")
	}
}

package sources

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/DeprecatedLuar/sat/internal/common"
)

const launcherTestAppID = "dev.test.App"

func TestWrapperTemplateUsesLauncherHelper(t *testing.T) {
	setupSourceBinEnv(t)
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("#!/usr/bin/env bash\nexec %s %s \"$@\"\n", common.FlatpakLauncherPath(), launcherTestAppID)
	if got := renderWrapper(launcherTestAppID); got != want {
		t.Fatalf("wrapper = %q, want %q", got, want)
	}
}

func TestWrapperRegexMatchesOldAndNewForms(t *testing.T) {
	setupSourceBinEnv(t)
	forms := []string{
		fmt.Sprintf(flatpakLegacyWrapperScript, launcherTestAppID),
		fmt.Sprintf(flatpakWrapperScript, common.FlatpakLauncherPath(), launcherTestAppID),
	}
	for _, f := range forms {
		m := flatpakWrapperAppIDRe.FindStringSubmatch(f)
		if len(m) < 2 || m[1] != launcherTestAppID {
			t.Fatalf("regex did not extract app ID from %q: %v", f, m)
		}
	}
}

func TestWrapperWithoutHelperFallsBackToDirectRun(t *testing.T) {
	setupSourceBinEnv(t)
	want := fmt.Sprintf(flatpakLegacyWrapperScript, launcherTestAppID)
	if got := renderWrapper(launcherTestAppID); got != want {
		t.Fatalf("wrapper = %q, want %q", got, want)
	}
}

func TestReconcileRewritesOldWrapperInPlace(t *testing.T) {
	setupSourceBinEnv(t)
	const name = "app"
	path := filepath.Join(common.FlatpakWrapperDir(), name)
	old := fmt.Sprintf(flatpakLegacyWrapperScript, launcherTestAppID)
	if err := os.WriteFile(path, []byte(old), 0755); err != nil {
		t.Fatal(err)
	}
	linkName, err := common.LinkBin(path)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(common.LocalBin(), linkName)
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}

	if reconcileWrapper(launcherTestAppID, name) {
		t.Fatal("reconcile reported a creation for an existing wrapper")
	}
	data, _ := os.ReadFile(path)
	if string(data) != renderWrapper(launcherTestAppID) {
		t.Fatalf("wrapper not rewritten: %q", data)
	}
	if target, err := os.Readlink(link); err != nil || target != path {
		t.Fatalf("link changed: %q, %v", target, err)
	}

	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	reconcileWrapper(launcherTestAppID, name)
	info, _ := os.Stat(path)
	if !info.ModTime().Equal(past) {
		t.Fatal("second reconcile rewrote an up-to-date wrapper")
	}
}

func TestLauncherHelperOutsideBinLinkOwnerDirs(t *testing.T) {
	setupSourceBinEnv(t)
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(common.FlatpakLauncherPath()) == common.FlatpakWrapperDir() {
		t.Fatal("helper lives in the wrapper dir")
	}
	common.ReconcileBinLinks()
	if _, err := os.Lstat(filepath.Join(common.LocalBin(), common.FlatpakLauncherName)); !os.IsNotExist(err) {
		t.Fatalf("helper got a bin link: %v", err)
	}
}

func TestEnsureLauncherHelperRewritesDriftOnly(t *testing.T) {
	setupSourceBinEnv(t)
	path := common.FlatpakLauncherPath()
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != flatpakLauncherScript {
		t.Fatal("drifted helper not rewritten")
	}

	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	if err := ensureLauncherHelper(); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); !info.ModTime().Equal(past) {
		t.Fatal("matching helper was rewritten")
	}
}

func TestLauncherScriptSyntax(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flatpak-launch")
	if err := os.WriteFile(path, []byte(flatpakLauncherScript), 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
}

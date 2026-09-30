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

func TestWrapperRegexExtractsAppID(t *testing.T) {
	setupSourceBinEnv(t)
	script := fmt.Sprintf(flatpakWrapperScript, common.FlatpakLauncherPath(), launcherTestAppID)
	m := flatpakWrapperAppIDRe.FindStringSubmatch(script)
	if len(m) < 2 || m[1] != launcherTestAppID {
		t.Fatalf("regex did not extract app ID from %q: %v", script, m)
	}
}

func TestCreateWrapperUsesLauncherHelper(t *testing.T) {
	setupSourceBinEnv(t)
	if err := createFlatpakWrapper("app", launcherTestAppID); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(common.FlatpakWrapperDir(), "app"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(flatpakWrapperScript, common.FlatpakLauncherPath(), launcherTestAppID)
	if string(data) != want {
		t.Fatalf("wrapper = %q, want %q", data, want)
	}
	if _, err := os.Stat(common.FlatpakLauncherPath()); err != nil {
		t.Fatalf("helper not written: %v", err)
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

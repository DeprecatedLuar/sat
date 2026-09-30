package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type binEnv struct {
	nix, system string
}

// setupBinEnv isolates HOME, SAT_DATA and PATH in temp dirs. PATH lists the
// fake ~/.local/bin twice, then a fake nix dir and a fake system dir.
func setupBinEnv(t *testing.T) binEnv {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("SAT_DATA", filepath.Join(root, "data"))

	env := binEnv{nix: filepath.Join(root, "nix"), system: filepath.Join(root, "system")}
	for _, d := range []string{LocalBin(), env.nix, env.system, AppImagesDir(), FlatpakWrapperDir(), HuberBinDir()} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{LocalBin(), env.nix, LocalBin(), env.system}, string(os.PathListSeparator)))
	return env
}

func dummyBin(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func linkTarget(t *testing.T, name string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(LocalBin(), name))
	if err != nil {
		return ""
	}
	return target
}

func mustLink(t *testing.T, target string) string {
	t.Helper()
	name, err := LinkBin(target)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func snapshotLocalBin(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, l := range mustLinks(t) {
		b.WriteString(l.name + "->" + l.target + "\n")
	}
	return b.String()
}

func mustLinks(t *testing.T) []binLink {
	t.Helper()
	links, err := localBinLinks()
	if err != nil {
		t.Fatal(err)
	}
	return links
}

func TestLinkBinNaming(t *testing.T) {
	env := setupBinEnv(t)

	t.Run("free name links naturally", func(t *testing.T) {
		target := dummyBin(t, AppImagesDir(), "free")
		if got := mustLink(t, target); got != "free" {
			t.Fatalf("got %q, want free", got)
		}
		if linkTarget(t, "free") != target {
			t.Fatal("link does not point at target")
		}
	})

	t.Run("foreign PATH dir forces suffix", func(t *testing.T) {
		dummyBin(t, env.nix, "krita")
		target := dummyBin(t, AppImagesDir(), "krita")
		if got := mustLink(t, target); got != "krita-appimage" {
			t.Fatalf("got %q, want krita-appimage", got)
		}
	})

	t.Run("later PATH dir is also detected", func(t *testing.T) {
		dummyBin(t, env.system, "late")
		target := dummyBin(t, AppImagesDir(), "late")
		if got := mustLink(t, target); got != "late-appimage" {
			t.Fatalf("got %q, want late-appimage", got)
		}
	})

	t.Run("own link is kept", func(t *testing.T) {
		target := dummyBin(t, AppImagesDir(), "own")
		mustLink(t, target)
		if got := mustLink(t, target); got != "own" {
			t.Fatalf("got %q, want own", got)
		}
	})

	t.Run("registry dir on PATH is not elsewhere", func(t *testing.T) {
		t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+AppImagesDir()+string(os.PathListSeparator)+HuberBinDir())
		target := dummyBin(t, AppImagesDir(), "regpath")
		if got := mustLink(t, target); got != "regpath" {
			t.Fatalf("got %q, want regpath", got)
		}
	})

	t.Run("both names taken errors", func(t *testing.T) {
		dummyBin(t, env.nix, "both")
		dummyBin(t, env.system, "both-appimage")
		target := dummyBin(t, AppImagesDir(), "both")
		if _, err := LinkBin(target); err == nil {
			t.Fatal("expected error when both names are taken")
		}
		if _, err := os.Lstat(filepath.Join(LocalBin(), "both")); err == nil {
			t.Fatal("link was created despite the error")
		}
	})

	t.Run("file outside registry errors", func(t *testing.T) {
		if _, err := LinkBin(dummyBin(t, env.nix, "x")); err == nil {
			t.Fatal("expected error for a non-owned directory")
		}
	})
}

func TestReconcileAddsAndDropsSuffix(t *testing.T) {
	env := setupBinEnv(t)
	target := dummyBin(t, AppImagesDir(), "krita")
	mustLink(t, target)

	foreign := dummyBin(t, env.nix, "krita")
	ReconcileBinLinks()
	if linkTarget(t, "krita-appimage") != target || linkTarget(t, "krita") != "" {
		t.Fatalf("expected suffixed link after foreign binary appeared, got:\n%s", snapshotLocalBin(t))
	}

	removeFile(t, foreign)
	ReconcileBinLinks()
	if linkTarget(t, "krita") != target || linkTarget(t, "krita-appimage") != "" {
		t.Fatalf("expected suffix dropped after foreign binary left, got:\n%s", snapshotLocalBin(t))
	}
}

func TestReconcileTwoSourcesSameNameIsStable(t *testing.T) {
	setupBinEnv(t)
	dummyBin(t, AppImagesDir(), "dup")
	dummyBin(t, FlatpakWrapperDir(), "dup")

	ReconcileBinLinks()
	first := snapshotLocalBin(t)
	if len(mustLinks(t)) != 2 {
		t.Fatalf("expected two links, got:\n%s", first)
	}
	if linkTarget(t, "dup") == "" || linkTarget(t, "dup-flatpak") == "" {
		t.Fatalf("expected dup and dup-flatpak, got:\n%s", first)
	}
	for i := 0; i < 3; i++ {
		ReconcileBinLinks()
		if got := snapshotLocalBin(t); got != first {
			t.Fatalf("reconcile %d changed links:\n%s\nwas:\n%s", i, got, first)
		}
	}
}

func TestReconcileRestoresHandRenamedLink(t *testing.T) {
	setupBinEnv(t)
	target := dummyBin(t, AppImagesDir(), "jackify")
	if err := os.Symlink(target, filepath.Join(LocalBin(), "jk")); err != nil {
		t.Fatal(err)
	}

	ReconcileBinLinks()
	links := mustLinks(t)
	if len(links) != 1 || links[0].name != "jackify" {
		t.Fatalf("hand-renamed link was not restored:\n%s", snapshotLocalBin(t))
	}
}

func TestReconcileRecreatesDeletedLinks(t *testing.T) {
	setupBinEnv(t)
	for _, dir := range []string{AppImagesDir(), FlatpakWrapperDir(), HuberBinDir()} {
		dummyBin(t, dir, filepath.Base(dir)+"tool")
	}

	ReconcileBinLinks()
	for _, name := range []string{"appimagestool", "flatpaktool", "bintool"} {
		if linkTarget(t, name) == "" {
			t.Fatalf("missing link %s:\n%s", name, snapshotLocalBin(t))
		}
	}
}

func TestReconcileRemovesDanglingOwnedLinksOnly(t *testing.T) {
	setupBinEnv(t)
	target := dummyBin(t, HuberBinDir(), "gone")
	mustLink(t, target)
	foreignDangling := filepath.Join(LocalBin(), "foreign")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), foreignDangling); err != nil {
		t.Fatal(err)
	}
	removeFile(t, target)

	ReconcileBinLinks()
	if linkTarget(t, "gone") != "" {
		t.Fatal("dangling owned link was kept")
	}
	if linkTarget(t, "foreign") == "" {
		t.Fatal("foreign dangling link was removed")
	}
}

func TestUnlinkBinRemovesSuffixedLinkOnly(t *testing.T) {
	env := setupBinEnv(t)
	dummyBin(t, env.nix, "tool")
	target := dummyBin(t, AppImagesDir(), "tool")
	if got := mustLink(t, target); got != "tool-appimage" {
		t.Fatalf("got %q", got)
	}
	foreign := filepath.Join(env.system, "tool")
	dummyBin(t, env.system, "tool")
	if err := os.Symlink(foreign, filepath.Join(LocalBin(), "tool")); err != nil {
		t.Fatal(err)
	}

	if err := UnlinkBin(target); err != nil {
		t.Fatal(err)
	}
	if linkTarget(t, "tool-appimage") != "" {
		t.Fatal("suffixed link was not removed")
	}
	if linkTarget(t, "tool") != foreign {
		t.Fatal("foreign link of the same name was removed")
	}
	if err := UnlinkBin(filepath.Join(AppImagesDir(), "missing")); err != nil {
		t.Fatalf("missing link should not error: %v", err)
	}
}

func TestReconcileWithNothingToChangePrintsNothing(t *testing.T) {
	env := setupBinEnv(t)
	dummyBin(t, env.nix, "krita")
	dummyBin(t, AppImagesDir(), "krita")
	dummyBin(t, HuberBinDir(), "plain")
	ReconcileBinLinks()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	ReconcileBinLinks()
	os.Stderr = orig
	w.Close()

	out := make([]byte, 1024)
	n, _ := r.Read(out)
	if n != 0 {
		t.Fatalf("reconcile printed: %s", out[:n])
	}
}

package sources

import (
	"os"
	"path/filepath"
	"testing"
)

// npmFixture builds a fake global npm layout under root:
//
//	root/bin/<binary>                -> symlink into node_modules
//	root/lib/node_modules/<pkg>/package.json
//
// mirroring the real npm global install layout this code depends on.
func npmFixture(t *testing.T, pkg, version string, bins ...string) (root string) {
	t.Helper()
	root = t.TempDir()

	pkgDir := filepath.Join(root, "lib", "node_modules", pkg)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", pkgDir, err)
	}
	pkgJSON := []byte(`{"name":"` + pkg + `","version":"` + version + `"}`)
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), pkgJSON, 0644); err != nil {
		t.Fatalf("WriteFile(package.json) error = %v", err)
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", binDir, err)
	}
	for _, bin := range bins {
		target := filepath.Join("..", "lib", "node_modules", pkg, "bin", bin+".js")
		if err := os.Symlink(target, filepath.Join(binDir, bin)); err != nil {
			t.Fatalf("Symlink(%s) error = %v", bin, err)
		}
	}
	return root
}

func TestNpmPackageDirFromBinPath(t *testing.T) {
	t.Run("scoped package", func(t *testing.T) {
		root := npmFixture(t, "@openai/codex", "0.153.4", "codex")
		binPath := filepath.Join(root, "bin", "codex")

		pkgDir, ok := npmPackageDirFromBinPath(binPath)
		if !ok {
			t.Fatalf("npmPackageDirFromBinPath(%s) ok = false, want true", binPath)
		}
		version := npmReadPackageVersion(pkgDir)
		if version != "0.153.4" {
			t.Errorf("resolved version = %q, want %q", version, "0.153.4")
		}
	})

	t.Run("unscoped package", func(t *testing.T) {
		root := npmFixture(t, "netlify-cli", "27.4.2", "netlify")
		binPath := filepath.Join(root, "bin", "netlify")

		pkgDir, ok := npmPackageDirFromBinPath(binPath)
		if !ok {
			t.Fatalf("npmPackageDirFromBinPath(%s) ok = false, want true", binPath)
		}
		version := npmReadPackageVersion(pkgDir)
		if version != "27.4.2" {
			t.Errorf("resolved version = %q, want %q", version, "27.4.2")
		}
	})

	t.Run("two binaries of one package resolve to the same version", func(t *testing.T) {
		root := npmFixture(t, "netlify-cli", "27.4.2", "netlify", "ntl")

		for _, bin := range []string{"netlify", "ntl"} {
			binPath := filepath.Join(root, "bin", bin)
			pkgDir, ok := npmPackageDirFromBinPath(binPath)
			if !ok {
				t.Fatalf("npmPackageDirFromBinPath(%s) ok = false, want true", binPath)
			}
			if v := npmReadPackageVersion(pkgDir); v != "27.4.2" {
				t.Errorf("bin %q resolved version = %q, want %q", bin, v, "27.4.2")
			}
		}
	})

	t.Run("broken symlink", func(t *testing.T) {
		root := t.TempDir()
		binPath := filepath.Join(root, "ghost")
		if err := os.Symlink(filepath.Join(root, "does-not-exist"), binPath); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}
		// Readlink succeeds even for a dangling symlink (it never follows
		// the target) - what should fail here is the regex match, since
		// the target has no node_modules/ segment at all.
		if _, ok := npmPackageDirFromBinPath(binPath); ok {
			t.Error("npmPackageDirFromBinPath() on a non-npm symlink target ok = true, want false")
		}
	})

	t.Run("non-symlink regular file", func(t *testing.T) {
		root := t.TempDir()
		binPath := filepath.Join(root, "plain")
		if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, ok := npmPackageDirFromBinPath(binPath); ok {
			t.Error("npmPackageDirFromBinPath() on a non-symlink ok = true, want false")
		}
	})

	t.Run("missing path", func(t *testing.T) {
		root := t.TempDir()
		if _, ok := npmPackageDirFromBinPath(filepath.Join(root, "nope")); ok {
			t.Error("npmPackageDirFromBinPath() on a missing path ok = true, want false")
		}
	})
}

func TestNpmReadPackageVersion(t *testing.T) {
	t.Run("missing version field", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"x"}`), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if v := npmReadPackageVersion(dir); v != "" {
			t.Errorf("npmReadPackageVersion() = %q, want empty", v)
		}
	})

	t.Run("missing package.json", func(t *testing.T) {
		dir := t.TempDir()
		if v := npmReadPackageVersion(dir); v != "" {
			t.Errorf("npmReadPackageVersion() = %q, want empty", v)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`not json`), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if v := npmReadPackageVersion(dir); v != "" {
			t.Errorf("npmReadPackageVersion() = %q, want empty", v)
		}
	})
}

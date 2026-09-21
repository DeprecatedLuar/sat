package common

import (
	"os"
	"path/filepath"
	"testing"
)

// setupBinLinkEnv points $HOME at a temp dir so LocalBin() is isolated, and
// returns two owner directories standing in for two different sources.
func setupBinLinkEnv(t *testing.T) (ownerA, ownerB string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(LocalBin(), 0755); err != nil {
		t.Fatal(err)
	}
	ownerA = filepath.Join(home, "a")
	ownerB = filepath.Join(home, "b")
	for _, d := range []string{ownerA, ownerB} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return ownerA, ownerB
}

func TestResolveBinName(t *testing.T) {
	ownerA, ownerB := setupBinLinkEnv(t)

	t.Run("free name is kept", func(t *testing.T) {
		got, err := ResolveBinName("tool", "a", ownerA)
		if err != nil || got != "tool" {
			t.Fatalf("got %q, %v; want tool", got, err)
		}
	})

	t.Run("own link is kept", func(t *testing.T) {
		if err := LinkBin("own", filepath.Join(ownerA, "own")); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveBinName("own", "a", ownerA)
		if err != nil || got != "own" {
			t.Fatalf("got %q, %v; want own", got, err)
		}
	})

	t.Run("other source's link forces suffix", func(t *testing.T) {
		if err := LinkBin("shared", filepath.Join(ownerB, "shared")); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveBinName("shared", "a", ownerA)
		if err != nil || got != "shared-a" {
			t.Fatalf("got %q, %v; want shared-a", got, err)
		}
	})

	t.Run("regular file forces suffix", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(LocalBin(), "script"), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
		got, err := ResolveBinName("script", "a", ownerA)
		if err != nil || got != "script-a" {
			t.Fatalf("got %q, %v; want script-a", got, err)
		}
	})

	t.Run("suffixed name also taken errors", func(t *testing.T) {
		if err := LinkBin("both", filepath.Join(ownerB, "both")); err != nil {
			t.Fatal(err)
		}
		if err := LinkBin("both-a", filepath.Join(ownerB, "both-a")); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveBinName("both", "a", ownerA); err == nil {
			t.Fatal("expected error when both names are taken")
		}
	})
}

func TestLinkBinIdempotentAndReplacesOwn(t *testing.T) {
	ownerA, _ := setupBinLinkEnv(t)
	target := filepath.Join(ownerA, "tool")

	if err := LinkBin("tool", target); err != nil {
		t.Fatal(err)
	}
	if err := LinkBin("tool", target); err != nil {
		t.Fatalf("relinking same target: %v", err)
	}

	newTarget := filepath.Join(ownerA, "tool2")
	if err := LinkBin("tool", newTarget); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(filepath.Join(LocalBin(), "tool")); got != newTarget {
		t.Fatalf("link points at %q, want %q", got, newTarget)
	}
}

func TestUnlinkBinOnlyRemovesOwnedLinks(t *testing.T) {
	ownerA, ownerB := setupBinLinkEnv(t)
	linkPath := filepath.Join(LocalBin(), "tool")

	if err := LinkBin("tool", filepath.Join(ownerB, "tool")); err != nil {
		t.Fatal(err)
	}
	if err := UnlinkBin("tool", ownerA); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatal("link owned by another source was removed")
	}

	if err := UnlinkBin("tool", ownerB); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatal("owned link was not removed")
	}

	if err := UnlinkBin("missing", ownerA); err != nil {
		t.Fatalf("missing link should not error: %v", err)
	}
}

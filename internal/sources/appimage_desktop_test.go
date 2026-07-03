package sources

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeTestELF(t *testing.T, class byte, eShoff uint64, eShentsize, eShnum uint16) string {
	t.Helper()

	header := make([]byte, 64)
	copy(header[:4], "\x7fELF")
	header[4] = class

	switch class {
	case 2: // 64-bit
		binary.LittleEndian.PutUint64(header[40:48], eShoff)
		binary.LittleEndian.PutUint16(header[58:60], eShentsize)
		binary.LittleEndian.PutUint16(header[60:62], eShnum)
	case 1: // 32-bit
		binary.LittleEndian.PutUint32(header[32:36], uint32(eShoff))
		binary.LittleEndian.PutUint16(header[46:48], eShentsize)
		binary.LittleEndian.PutUint16(header[48:50], eShnum)
	}

	path := filepath.Join(t.TempDir(), "test.AppImage")
	if err := os.WriteFile(path, header, 0644); err != nil {
		t.Fatalf("failed to write test ELF: %v", err)
	}
	return path
}

func TestElfSquashfsOffset64Bit(t *testing.T) {
	// Mirrors the real hydralauncher AppImage verified during planning:
	// e_shoff=188200, e_shentsize=64, e_shnum=3 -> offset 188392.
	path := writeTestELF(t, 2, 188200, 64, 3)

	offset, err := elfSquashfsOffset(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := int64(188392); offset != want {
		t.Errorf("got offset %d, want %d", offset, want)
	}
}

func TestElfSquashfsOffset32Bit(t *testing.T) {
	path := writeTestELF(t, 1, 5000, 40, 10)

	offset, err := elfSquashfsOffset(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := int64(5400); offset != want {
		t.Errorf("got offset %d, want %d", offset, want)
	}
}

func TestElfSquashfsOffsetNotELF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-elf")
	if err := os.WriteFile(path, []byte("not an elf file at all"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if _, err := elfSquashfsOffset(path); err == nil {
		t.Error("expected error for non-ELF input, got nil")
	}
}

func TestParseDesktopFile(t *testing.T) {
	// Actual content extracted from the real hydralauncher AppImage during
	// manual planning verification.
	content := `[Desktop Entry]
Name=Hydra
Exec=AppRun --no-sandbox %U
Terminal=false
Type=Application
Icon=hydralauncher
StartupWMClass=Hydra
X-AppImage-Version=4.0.4
Comment=Hydra
MimeType=x-scheme-handler/hydralauncher;
Categories=Game;
`
	path := filepath.Join(t.TempDir(), "hydralauncher.desktop")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test .desktop file: %v", err)
	}

	fields, err := parseDesktopFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"Name":               "Hydra",
		"Exec":               "AppRun --no-sandbox %U",
		"Terminal":           "false",
		"Type":               "Application",
		"Icon":               "hydralauncher",
		"StartupWMClass":     "Hydra",
		"X-AppImage-Version": "4.0.4",
		"Comment":            "Hydra",
		"MimeType":           "x-scheme-handler/hydralauncher;",
		"Categories":         "Game;",
	}
	for k, v := range want {
		if fields[k] != v {
			t.Errorf("field %q: got %q, want %q", k, fields[k], v)
		}
	}
}

func TestRewriteExec(t *testing.T) {
	cases := []struct {
		name         string
		originalExec string
		symlinkPath  string
		want         string
	}{
		{
			name:         "preserves field code",
			originalExec: "AppRun --no-sandbox %U",
			symlinkPath:  "/home/user/.local/bin/hydralauncher",
			want:         "/home/user/.local/bin/hydralauncher %U",
		},
		{
			name:         "no field codes",
			originalExec: "AppRun --headless",
			symlinkPath:  "/home/user/.local/bin/foo",
			want:         "/home/user/.local/bin/foo",
		},
		{
			name:         "multiple field codes",
			originalExec: "AppRun %f %U",
			symlinkPath:  "/bin/bar",
			want:         "/bin/bar %f %U",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rewriteExec(c.originalExec, c.symlinkPath)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveIconPath(t *testing.T) {
	t.Run("png found", func(t *testing.T) {
		dir := t.TempDir()
		iconPath := filepath.Join(dir, "myapp.png")
		os.WriteFile(iconPath, []byte("fake png"), 0644)

		got, err := resolveIconPath(dir, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != iconPath {
			t.Errorf("got %q, want %q", got, iconPath)
		}
	})

	t.Run("svg found", func(t *testing.T) {
		dir := t.TempDir()
		iconPath := filepath.Join(dir, "myapp.svg")
		os.WriteFile(iconPath, []byte("fake svg"), 0644)

		got, err := resolveIconPath(dir, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != iconPath {
			t.Errorf("got %q, want %q", got, iconPath)
		}
	})

	t.Run("falls back to DirIcon", func(t *testing.T) {
		dir := t.TempDir()
		iconPath := filepath.Join(dir, ".DirIcon")
		os.WriteFile(iconPath, []byte("fake icon"), 0644)

		got, err := resolveIconPath(dir, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != iconPath {
			t.Errorf("got %q, want %q", got, iconPath)
		}
	})

	t.Run("nothing found returns empty, not error", func(t *testing.T) {
		dir := t.TempDir()

		got, err := resolveIconPath(dir, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}

package sources

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DeprecatedLuar/sat/internal/common"
	"github.com/DeprecatedLuar/sat/internal/manifest"
	"github.com/DeprecatedLuar/sat/internal/ui"
)

// desktopSkipMarkerSuffix names the marker file written when an AppImage was
// extracted successfully but had no usable .desktop entry inside it (bad
// packaging, unsupported compression, type1 image). Its presence tells
// selfheal's backfill loop not to re-run extraction on every invocation.
const desktopSkipMarkerSuffix = ".skip"

// iconCandidateExts are the icon file extensions checked next to a resolved
// Icon= name, in priority order.
var iconCandidateExts = []string{".png", ".svg", ".xpm"}

// elfSquashfsOffset returns the byte offset where a type2 AppImage's
// appended squashfs filesystem begins, computed the same way AppImageKit's
// own runtime does: the end of the ELF section header table
// (e_shoff + e_shentsize*e_shnum). Verified against a real installed
// AppImage during planning.
func elfSquashfsOffset(appimagePath string) (int64, error) {
	f, err := os.Open(appimagePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	header := make([]byte, 64)
	if _, err := io.ReadFull(f, header); err != nil {
		return 0, fmt.Errorf("failed to read ELF header: %w", err)
	}
	if string(header[:4]) != "\x7fELF" {
		return 0, fmt.Errorf("not an ELF file")
	}

	eiClass := header[4]
	var eShoff int64
	var eShentsize, eShnum int64
	switch eiClass {
	case 2: // 64-bit
		eShoff = int64(binary.LittleEndian.Uint64(header[40:48]))
		eShentsize = int64(binary.LittleEndian.Uint16(header[58:60]))
		eShnum = int64(binary.LittleEndian.Uint16(header[60:62]))
	case 1: // 32-bit
		eShoff = int64(binary.LittleEndian.Uint32(header[32:36]))
		eShentsize = int64(binary.LittleEndian.Uint16(header[46:48]))
		eShnum = int64(binary.LittleEndian.Uint16(header[48:50]))
	default:
		return 0, fmt.Errorf("unrecognized ELF class %d", eiClass)
	}

	return eShoff + eShentsize*eShnum, nil
}

// extractSquashfsPayload extracts the squashfs filesystem appended to a
// type2 AppImage at offset into destDir, without ever executing the
// AppImage itself.
func extractSquashfsPayload(appimagePath string, offset int64, destDir string) error {
	return common.RunQuiet("unsquashfs", "-o", strconv.FormatInt(offset, 10), "-d", destDir, appimagePath)
}

// findEmbeddedDesktopFile returns the path to the first *.desktop file
// found directly under rootDir, per the AppImage spec's requirement that
// one live at the squashfs root.
func findEmbeddedDesktopFile(rootDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(rootDir, "*.desktop"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no .desktop file found")
	}
	return matches[0], nil
}

// parseDesktopFile parses a freedesktop .desktop file's [Desktop Entry]
// section into a flat key=value map. Deliberately simple (no INI library)
// since sat only needs a handful of well-known fields.
func parseDesktopFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return fields, nil
}

// resolveIconPath finds the icon file referenced by a .desktop entry's
// Icon= value within an extracted AppImage root. Returns "" (not an error)
// if no icon can be found — an icon-less desktop entry is a valid outcome.
func resolveIconPath(rootDir, iconName string) (string, error) {
	if iconName != "" {
		for _, ext := range iconCandidateExts {
			candidate := filepath.Join(rootDir, iconName+ext)
			if fileExists(candidate) {
				return candidate, nil
			}
		}
	}

	dirIcon := filepath.Join(rootDir, ".DirIcon")
	if fileExists(dirIcon) {
		return dirIcon, nil
	}

	return "", nil
}

// execFieldCodes are the freedesktop Exec= field codes that must be
// preserved verbatim when rewriting the command.
var execFieldCodes = map[string]bool{
	"%f": true, "%F": true, "%u": true, "%U": true,
	"%i": true, "%c": true, "%k": true,
}

// rewriteExec replaces the leading command of a .desktop Exec= value with
// symlinkPath, preserving any trailing field codes from the original.
func rewriteExec(originalExec, symlinkPath string) string {
	fields := strings.Fields(originalExec)

	var codes []string
	for _, f := range fields[1:] {
		if execFieldCodes[f] {
			codes = append(codes, f)
		}
	}

	if len(codes) == 0 {
		return symlinkPath
	}
	return symlinkPath + " " + strings.Join(codes, " ")
}

// writeDesktopEntry builds and writes the canonical .desktop file for an
// installed AppImage, using the parsed embedded fields with sat-specific
// overrides for Exec=, Icon=, and provenance.
func writeDesktopEntry(fields map[string]string, appName, symlinkPath, iconPath, repoPath string) error {
	name := fields["Name"]
	if name == "" {
		name = appName
	}

	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	fmt.Fprintf(&b, "Name=%s\n", name)
	if comment := fields["Comment"]; comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", comment)
	}
	fmt.Fprintf(&b, "Exec=%s\n", rewriteExec(fields["Exec"], symlinkPath))
	if iconPath != "" {
		fmt.Fprintf(&b, "Icon=%s\n", iconPath)
	}
	b.WriteString("Terminal=false\n")
	if categories := fields["Categories"]; categories != "" {
		fmt.Fprintf(&b, "Categories=%s\n", categories)
	}
	b.WriteString("Type=Application\n")
	if repoPath != "" {
		fmt.Fprintf(&b, "X-AppImage-Source=%s\n", repoPath)
	}

	dir := common.AppImageApplicationsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, appName+".desktop"), []byte(b.String()), 0644)
}

// skipMarkerPath returns the path to appName's .skip marker.
func skipMarkerPath(appName string) string {
	return filepath.Join(common.AppImageApplicationsDir(), appName+desktopSkipMarkerSuffix)
}

// writeSkipMarker records that appName's AppImage was extracted but had no
// usable desktop entry, so future selfheal runs don't retry extraction.
func writeSkipMarker(appName string) {
	dir := common.AppImageApplicationsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	_ = os.WriteFile(skipMarkerPath(appName), nil, 0644)
}

// BackfillDesktopEntries generates desktop entries for AppImages installed
// before desktop-entry generation existed (or installed while unsquashfs
// was unavailable). Idempotent and local-only: skips any tool that already
// has an entry or has been marked permanently skippable via
// HasDesktopEntry, so the common case is one cheap stat per installed
// AppImage. Called by selfheal on every invocation.
func BackfillDesktopEntries() {
	appimagesDir := common.AppImagesDir()
	entries, err := os.ReadDir(appimagesDir)
	if err != nil {
		return
	}

	skipDirs := map[string]bool{
		common.AppImageApplicationsDirName: true,
		common.AppImageIconsDirName:        true,
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || skipDirs[name] {
			continue
		}
		if HasDesktopEntry(name) {
			continue
		}

		appimagePath := filepath.Join(appimagesDir, name)
		symlinkPath := filepath.Join(common.LocalBin(), name)
		repoPath := manifest.GetSourceIdentity(manifest.Get(name))

		InstallDesktopEntry(appimagePath, symlinkPath, name, repoPath, false)
	}
}

// HasDesktopEntry reports whether appName already has a generated desktop
// entry, or has been marked as permanently skippable. Used by selfheal's
// backfill loop to avoid redundant extraction work.
func HasDesktopEntry(appName string) bool {
	desktopPath := filepath.Join(common.AppImageApplicationsDir(), appName+".desktop")
	return fileExists(desktopPath) || fileExists(skipMarkerPath(appName))
}

// InstallDesktopEntry generates a desktop entry and icon for an installed
// AppImage, symlinking the entry into the standard XDG applications
// directory so it appears in application launchers. Never returns an error
// that should fail an install or backfill — all failure paths are logged
// (via SAT_DEBUG or ui.Warn) and treated as a clean skip. warnMissingTool
// controls whether a missing unsquashfs surfaces a ui.Warn: true for a
// fresh AppImage install (the user just acted, so it's actionable now),
// false for selfheal's backfill loop (which retries every startup and
// would otherwise repeat the same warning on every sat invocation).
func InstallDesktopEntry(appimagePath, symlinkPath, appName, repoPath string, warnMissingTool bool) {
	debug := os.Getenv(common.EnvSATDebug) != ""

	if _, err := exec.LookPath("unsquashfs"); err != nil {
		if warnMissingTool {
			ui.Warn(fmt.Sprintf("desktop entry not created for %s (unsquashfs not found — install squashfs-tools for app-launcher icons)", appName))
		} else if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: unsquashfs not found, skipping %s\n", appName)
		}
		return
	}

	offset, err := elfSquashfsOffset(appimagePath)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to compute squashfs offset: %v\n", err)
		}
		writeSkipMarker(appName)
		return
	}

	tmpDir, err := os.MkdirTemp("", "sat-appimage-extract-*")
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to create temp dir: %v\n", err)
		}
		return
	}
	defer os.RemoveAll(tmpDir)

	rootDir := filepath.Join(tmpDir, "root")
	if err := extractSquashfsPayload(appimagePath, offset, rootDir); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: squashfs extraction failed: %v\n", err)
		}
		writeSkipMarker(appName)
		return
	}

	desktopFile, err := findEmbeddedDesktopFile(rootDir)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: %v\n", err)
		}
		writeSkipMarker(appName)
		return
	}

	fields, err := parseDesktopFile(desktopFile)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to parse .desktop file: %v\n", err)
		}
		writeSkipMarker(appName)
		return
	}

	iconSrc, err := resolveIconPath(rootDir, fields["Icon"])
	if err != nil && debug {
		fmt.Fprintf(os.Stderr, "[debug]   desktop entry: icon resolution failed: %v\n", err)
	}

	var iconDest string
	if iconSrc != "" {
		iconDir := common.AppImageIconsDir()
		if err := os.MkdirAll(iconDir, 0755); err == nil {
			iconDest = filepath.Join(iconDir, appName+filepath.Ext(iconSrc))
			if err := copyFile(iconSrc, iconDest); err != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[debug]   desktop entry: icon copy failed: %v\n", err)
				}
				iconDest = ""
			}
		}
	}

	if err := writeDesktopEntry(fields, appName, symlinkPath, iconDest, repoPath); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to write .desktop file: %v\n", err)
		}
		return
	}

	appsDir := common.DesktopApplicationsDir()
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to create %s: %v\n", appsDir, err)
		}
		return
	}
	linkPath := filepath.Join(appsDir, "sat-"+appName+".desktop")
	os.Remove(linkPath)
	if err := os.Symlink(filepath.Join(common.AppImageApplicationsDir(), appName+".desktop"), linkPath); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[debug]   desktop entry: failed to symlink into %s: %v\n", appsDir, err)
		}
		return
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[debug]   desktop entry created for %s\n", appName)
	}
}

// removeDesktopEntry removes appName's desktop entry symlink, canonical
// .desktop file, skip marker, and icon, if present. Matches the
// os.IsNotExist-tolerant semantics of AppImageUninstall.
func removeDesktopEntry(appName string) {
	os.Remove(filepath.Join(common.DesktopApplicationsDir(), "sat-"+appName+".desktop"))
	os.Remove(filepath.Join(common.AppImageApplicationsDir(), appName+".desktop"))
	os.Remove(skipMarkerPath(appName))

	if matches, err := filepath.Glob(filepath.Join(common.AppImageIconsDir(), appName+".*")); err == nil {
		for _, m := range matches {
			os.Remove(m)
		}
	}
}

// copyFile copies src to dst, creating/truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

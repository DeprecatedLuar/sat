package common

import (
	"strconv"
	"strings"
)

// versionComponents splits a version string into its numeric components,
// tolerating a leading "v" and any run of non-digit characters as a
// separator (covers "." and "-" build-tag suffixes alike). ok is false if
// no numeric component could be extracted at all.
func versionComponents(v string) (parts []int, ok bool) {
	v = strings.TrimPrefix(v, "v")
	var num strings.Builder
	flush := func() {
		if num.Len() == 0 {
			return
		}
		n, err := strconv.Atoi(num.String())
		if err == nil {
			parts = append(parts, n)
		}
		num.Reset()
	}
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9':
			num.WriteRune(r)
		case r == '.' || r == '-':
			flush()
		default:
			// Any other character (letters, etc.) means this isn't a
			// plain numeric version string - bail out entirely rather
			// than treat an embedded digit run as a component.
			return nil, false
		}
	}
	flush()
	return parts, len(parts) > 0
}

// CompareVersions reports whether a is older (-1), equal (0) or newer (1)
// than b, and whether the comparison could be made at all.
//
// This is the fail-CLOSED counterpart to VersionIsNewer. Update checks
// want to fail open, since surfacing a maybe-update is harmless. Choosing
// between competing packages is the opposite: an undecidable comparison
// must not hand the win to whichever side happened to be unparseable, so
// callers fall back to an explicit preference order instead.
//
// Packaging noise is stripped first - Debian epochs and revisions
// ("1:1.6-2.1+deb12u2"), Ubuntu suffixes ("1.8.1-4ubuntu2") and tag
// prefixes ("v1.2.3", "jq-1.8.2") all describe the package, not the
// upstream release.
func CompareVersions(a, b string) (int, bool) {
	aParts, aOK := comparableComponents(a)
	bParts, bOK := comparableComponents(b)
	if !aOK || !bOK {
		return 0, false
	}

	for i := 0; i < len(aParts) || i < len(bParts); i++ {
		var x, y int
		if i < len(aParts) {
			x = aParts[i]
		}
		if i < len(bParts) {
			y = bParts[i]
		}
		if x != y {
			if x > y {
				return 1, true
			}
			return -1, true
		}
	}
	return 0, true
}

// comparableComponents normalizes a packaged version down to the upstream
// release numbers it describes.
func comparableComponents(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "-" {
		return nil, false
	}

	if _, rest, found := strings.Cut(v, ":"); found {
		v = rest // drop Debian epoch
	}

	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if head, rest, found := strings.Cut(v, "-"); found && !strings.ContainsAny(head, "0123456789") {
		v = strings.TrimPrefix(rest, "v") // "jq-1.8.2" -> "1.8.2"
	}

	// Everything from the packaging revision or build metadata onward
	// belongs to the package, not the release being compared.
	for _, sep := range []string{"-", "+", "~", "_"} {
		if head, _, found := strings.Cut(v, sep); found {
			v = head
		}
	}

	var parts []int
	for _, field := range strings.Split(v, ".") {
		n, err := strconv.Atoi(field)
		if err != nil {
			break // a trailing rc1/beta ends the numeric prefix
		}
		parts = append(parts, n)
	}

	if len(parts) == 0 {
		return nil, false
	}
	return parts, true
}

// VersionIsNewer reports whether latest is strictly newer than current,
// comparing numeric components split on any non-digit separator (v-prefix
// tolerated, e.g. "v1.2.3", "1.2.3-4"). Missing trailing components are
// treated as 0, so "1.3" > "1.2.9" and "1.2.1" > "1.2".
//
// If either side has no parseable numeric component, this returns true
// (do-not-suppress) so an update sat can't confidently order (git hashes,
// date tags, etc.) is never hidden - conservative in favor of surfacing.
func VersionIsNewer(latest, current string) bool {
	latestParts, latestOK := versionComponents(latest)
	currentParts, currentOK := versionComponents(current)
	if !latestOK || !currentOK {
		return true
	}

	for i := 0; i < len(latestParts) || i < len(currentParts); i++ {
		var l, c int
		if i < len(latestParts) {
			l = latestParts[i]
		}
		if i < len(currentParts) {
			c = currentParts[i]
		}
		if l != c {
			return l > c
		}
	}
	return false
}

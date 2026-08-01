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

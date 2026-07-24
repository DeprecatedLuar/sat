package manifest

import "strings"

const (
	// Source string format constants
	SourceDelimiter     = ":"
	SourceFieldCount    = 3 // source:identity:version
	SourceIdentityIndex = 1
	SourceVersionIndex  = 2
)

// BuildSourceString constructs a source string from components
// Format: source:identity:version
func BuildSourceString(source, identity, version string) string {
	return source + SourceDelimiter + identity + SourceDelimiter + version
}

// GetSourceType extracts the source type (first field)
func GetSourceType(s string) string {
	if idx := strings.Index(s, SourceDelimiter); idx != -1 {
		return s[:idx]
	}
	return s
}

// GetSourceIdentity extracts the identity (second field)
func GetSourceIdentity(s string) string {
	parts := strings.SplitN(s, SourceDelimiter, SourceFieldCount)
	if len(parts) >= SourceIdentityIndex+1 {
		return parts[SourceIdentityIndex]
	}
	return ""
}

// GetSourceVersion extracts the version (third field)
func GetSourceVersion(s string) string {
	parts := strings.SplitN(s, SourceDelimiter, SourceFieldCount)
	if len(parts) >= SourceVersionIndex+1 {
		return parts[SourceVersionIndex]
	}
	return ""
}

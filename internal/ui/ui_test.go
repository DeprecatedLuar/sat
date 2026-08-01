package ui

import (
	"strings"
	"testing"
)

func TestSourceDisplay(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::14.1.0", "rust"},
		{"npm::1.0.0", "node"},
		{"uv::0.5.0", "python"},
		{"pacman::", "system"},
		{"gh:owner/repo:v1.0", "github"},
		{"brew::5.0", "brew"},
	}

	for _, tt := range tests {
		got := SourceDisplay(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceDisplay(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}

func TestSourceColor(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::1.0", Rust},
		{"npm::1.0", Node},
		{"uv::1.0", Python},
		{"system::", System},
		{"unknown::", Dim},
	}

	for _, tt := range tests {
		got := SourceColor(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceColor(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}

// TestSourceLight locks in the fix for a prior drift bug: SourceLight was
// missing the "unknown" case that SourceColor/SourceDisplay had, and would
// silently fall back to Reset instead of a defined light color.
func TestSourceLight(t *testing.T) {
	tests := []struct {
		sourceStr string
		want      string
	}{
		{"cargo::1.0", RustLight},
		{"npm::1.0", NodeLight},
		{"system::", SystemLight},
		{"unknown::", Reset},
	}

	for _, tt := range tests {
		got := SourceLight(tt.sourceStr)
		if got != tt.want {
			t.Errorf("SourceLight(%q) = %q, want %q", tt.sourceStr, got, tt.want)
		}
	}
}

// TestStatusLineConsistentLayout locks in the fix for the spinner-vs-completion
// reflow bug: the padded name and source tag must render identically no
// matter which marker (spinner frame, check, cross) is swapped in, so a line
// never shifts between "in progress" and "done".
func TestStatusLineConsistentLayout(t *testing.T) {
	tail := func(line string) string {
		// Layout after the marker is identical regardless of marker content;
		// strip everything through the first "] " to compare just that tail.
		if i := strings.Index(line, "] "); i != -1 {
			return line[i:]
		}
		return line
	}

	spinnerFrame := frameColors[0] + frames[0] + Reset
	nameStyle := SourceLight("npm::1.0.0")
	spinning := statusLine(spinnerFrame, nameStyle, "claude", "npm::1.0.0")
	success := statusLine(check, nameStyle, "claude", "npm::1.0.0")
	failure := statusLine(cross, nameStyle, "claude", "npm::1.0.0")

	if tail(spinning) != tail(success) {
		t.Errorf("spinner and success lines diverge after the marker:\n  spinning: %q\n  success:  %q", spinning, success)
	}
	if tail(spinning) != tail(failure) {
		t.Errorf("spinner and failure lines diverge after the marker:\n  spinning: %q\n  failure:  %q", spinning, failure)
	}
}

func TestFormatReason(t *testing.T) {
	if got := formatReason("", 80); got != "" {
		t.Errorf("formatReason(%q) = %q, want empty", "", got)
	}

	got := formatReason("not found", 80)
	want := Dim + reasonIndent + "not found" + Reset
	if got != want {
		t.Errorf("formatReason(short) = %q, want %q", got, want)
	}

	got = formatReason("this reason is deliberately long enough to wrap onto a second line", 30)
	if !strings.Contains(got, "\n"+reasonContinueIndent) {
		t.Errorf("formatReason(long) did not wrap: %q", got)
	}
	if !strings.HasPrefix(got, Dim+reasonIndent) || !strings.HasSuffix(got, Reset) {
		t.Errorf("formatReason(long) missing Dim/Reset wrapper: %q", got)
	}
}

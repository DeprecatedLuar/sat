package sources

import (
	"reflect"
	"testing"
)

func TestParseBrewListVersions(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name:   "typical formulae and casks",
			output: "fd 10.3.0\nripgrep 14.1.1\n",
			want:   map[string]string{"fd": "10.3.0", "ripgrep": "14.1.1"},
		},
		{
			name:   "formula with two installed versions on one line",
			output: "python@3.12 3.12.1 3.12.3\n",
			want:   map[string]string{"python@3.12": "3.12.3"},
		},
		{
			name:   "empty output",
			output: "",
			want:   map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBrewListVersions(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBrewListVersions(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

func TestParseDpkgList(t *testing.T) {
	output := `Desired=Unknown/Install/Remove/Purge/Hold
| Status=Not/Inst/Conf-files/Unpacked/halF-conf/Half-inst/trig-aWait/Trig-pend
|/ Err?=(none)/Reinst-required (Status,Err: uppercase=bad)
||/ Name           Version      Architecture Description
+++-==============-============-============-=================================
ii  ripgrep        14.1.1       amd64        recursively search directories
rc  old-package    1.0.0        amd64        removed but config remains
ii  fd-find        10.3.0       amd64        simple, fast alternative to find
`
	want := map[string]string{
		"ripgrep": "14.1.1",
		"fd-find": "10.3.0",
	}
	got := parseDpkgList(output)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDpkgList() = %v, want %v", got, want)
	}
}

func TestParsePacmanQ(t *testing.T) {
	output := "fd 10.3.0-1\nripgrep 14.1.1-1\n"
	want := map[string]string{
		"fd":      "10.3.0-1",
		"ripgrep": "14.1.1-1",
	}
	got := parsePacmanQ(output)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePacmanQ() = %v, want %v", got, want)
	}
}

func TestParseDnfListInstalled(t *testing.T) {
	output := `Installed Packages
ripgrep.x86_64              14.1.1-1.fc39           @updates
fd-find.x86_64              10.3.0-1.fc39           @fedora
`
	want := map[string]string{
		"ripgrep": "14.1.1-1.fc39",
		"fd-find": "10.3.0-1.fc39",
	}
	got := parseDnfListInstalled(output)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDnfListInstalled() = %v, want %v", got, want)
	}
}

func TestParseApkInfoV(t *testing.T) {
	output := "ripgrep-14.1.1-r0\nfd-10.3.0-r1\n"
	tests := []struct {
		name  string
		tools []string
		want  map[string]string
	}{
		{
			name:  "exact matches",
			tools: []string{"ripgrep", "fd"},
			want:  map[string]string{"ripgrep": "14.1.1-r0", "fd": "10.3.0-r1"},
		},
		{
			name:  "unrequested package not included",
			tools: []string{"fd"},
			want:  map[string]string{"fd": "10.3.0-r1"},
		},
		{
			name:  "tool not installed",
			tools: []string{"nonexistent"},
			want:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseApkInfoV(output, tt.tools)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseApkInfoV(%v) = %v, want %v", tt.tools, got, tt.want)
			}
		})
	}
}

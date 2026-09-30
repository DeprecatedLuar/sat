package sources

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoModulePath(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"plain", "module github.com/a/b\n\ngo 1.21\n", "github.com/a/b", false},
		{"comment", "module charm.land/glow/v3 // vanity\n", "charm.land/glow/v3", false},
		{"quoted", "module \"example.com/x\"\n", "example.com/x", false},
		{"lowercase differs from repo", "module github.com/cladamos/solcl\n", "github.com/cladamos/solcl", false},
		{"missing", "go 1.21\n", "", true},
	}
	for _, tt := range tests {
		got, err := parseGoModulePath(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: got (%q, %v), want %q (err=%v)", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestIsGoModulePath(t *testing.T) {
	for spec, want := range map[string]bool{
		"charm.land/glow/v3":   true,
		"github.com/a/b":       true,
		"mikefarah/yq":         false,
		"sqlc-dev/sqlc":        false,
		"some.user/with-a-dot": true,
	} {
		if got := isGoModulePath(spec); got != want {
			t.Errorf("isGoModulePath(%q) = %v, want %v", spec, got, want)
		}
	}
}

func TestGoBaseName(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/mikefarah/yq/v4":        "yq",
		"charm.land/glow/v3":                "glow",
		"github.com/sqlc-dev/sqlc/cmd/sqlc": "sqlc",
		"github.com/x/cmd":                  "cmd",
		"v2":                                "v2",
	} {
		if got := goBaseName(in); got != want {
			t.Errorf("goBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSelectGoMainPackage(t *testing.T) {
	tests := []struct {
		name, module, repo string
		pkgs               []string
		want               string
		wantAmbiguous      []string
		wantErr            bool
	}{
		{
			name: "root main package with vN module", module: "github.com/mikefarah/yq/v4", repo: "yq",
			pkgs: []string{"github.com/mikefarah/yq/v4"},
			want: "github.com/mikefarah/yq/v4",
		},
		{
			name: "repo name beats sibling generators", module: "github.com/sqlc-dev/sqlc", repo: "sqlc",
			pkgs: []string{
				"github.com/sqlc-dev/sqlc/cmd/sqlc-gen-json",
				"github.com/sqlc-dev/sqlc/cmd/sqlc",
			},
			want: "github.com/sqlc-dev/sqlc/cmd/sqlc",
		},
		{
			name: "helper paths leave one candidate", module: "github.com/cli/cli/v2", repo: "cli",
			pkgs: []string{
				"github.com/cli/cli/v2/cmd/gh",
				"github.com/cli/cli/v2/cmd/gen-docs",
				"github.com/cli/cli/v2/internal/tools/x",
				"github.com/cli/cli/v2/script/y",
			},
			want: "github.com/cli/cli/v2/cmd/gh",
		},
		{
			name: "generic cmd dir", module: "github.com/u/cyberspace-tui-go", repo: "cyberspace-tui-go",
			pkgs: []string{"github.com/u/cyberspace-tui-go/cmd"},
			want: "github.com/u/cyberspace-tui-go/cmd",
		},
		{
			name: "module base beats root when repo differs", module: "charm.land/glow/v3", repo: "other",
			pkgs: []string{"charm.land/glow/v3", "charm.land/glow/v3/cmd/zzz"},
			want: "charm.land/glow/v3",
		},
		{
			name: "two unrelated", module: "github.com/u/tool", repo: "tool",
			pkgs:          []string{"github.com/u/tool/cmd/a", "github.com/u/tool/cmd/b"},
			wantAmbiguous: []string{"github.com/u/tool/cmd/a", "github.com/u/tool/cmd/b"},
		},
		{
			name: "only helpers", module: "github.com/u/tool", repo: "tool",
			pkgs:    []string{"github.com/u/tool/internal/x"},
			wantErr: true,
		},
		{
			name: "no main packages", module: "github.com/u/lib", repo: "lib", wantErr: true,
		},
	}
	for _, tt := range tests {
		got, err := selectGoMainPackage(tt.module, tt.repo, tt.pkgs)
		if tt.wantAmbiguous != nil {
			var ambig *AmbiguousGoPackageError
			if !errors.As(err, &ambig) || strings.Join(ambig.Candidates, ",") != strings.Join(tt.wantAmbiguous, ",") {
				t.Errorf("%s: err = %v, want ambiguous %v", tt.name, err, tt.wantAmbiguous)
			}
			continue
		}
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: got (%q, %v), want %q (err=%v)", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestGoBinaryName(t *testing.T) {
	for _, tt := range []struct{ produced, repo, want string }{
		{"cmd", "cyberspace-tui-go", "cyberspace-tui-go"},
		{"main", "tool", "tool"},
		{"gh", "cli", "gh"},
		{"sqlc", "sqlc", "sqlc"},
	} {
		if got := goBinaryName(tt.produced, tt.repo); got != tt.want {
			t.Errorf("goBinaryName(%q,%q) = %q, want %q", tt.produced, tt.repo, got, tt.want)
		}
	}
}

func TestNestedGoModPath(t *testing.T) {
	tree := []string{
		"README.md",
		"api/go.mod",
		"kustomize/go.mod",
		"plugin/kustomize/go.mod",
		"cmd/config/go.mod",
	}
	if got := nestedGoModPath(tree, "kustomize"); got != "kustomize/go.mod" {
		t.Errorf("got %q, want kustomize/go.mod", got)
	}
	if got := nestedGoModPath(tree, "absent"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := nestedGoModPath([]string{"go.mod"}, "go"); got != "" {
		t.Errorf("root go.mod must not count as nested, got %q", got)
	}
}

func TestParseGoModVersion(t *testing.T) {
	out := "/bin/x: go1.25.0\n\tpath\tgithub.com/a/b/cmd/x\n\tmod\tgithub.com/a/b\tv1.31.1\th1:abc=\n\tdep\tfoo\tv1\th1:z=\n"
	if got := parseGoModVersion(out); got != "v1.31.1" {
		t.Errorf("got %q, want v1.31.1", got)
	}
	devel := "\tmod\tgithub.com/a/b\t(devel)\t\n"
	if got := parseGoModVersion(devel); got != "" {
		t.Errorf("devel build should report empty, got %q", got)
	}
	if got := parseGoModVersion("no mod line"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func rawServer(t *testing.T, files map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := goRawBase
	goRawBase = srv.URL
	t.Cleanup(func() { goRawBase = prev })
}

func TestFetchRawGoMod_NotFoundIsSentinel(t *testing.T) {
	rawServer(t, map[string]string{})
	if _, err := fetchRawGoMod("o/r", goModFile); !errors.Is(err, errGoModNotFound) {
		t.Errorf("err = %v, want errGoModNotFound", err)
	}
}

func TestResolveGoModule(t *testing.T) {
	rawServer(t, map[string]string{
		"/Cladamos/solcl/HEAD/go.mod": "module github.com/cladamos/solcl\n",
	})

	module, repo, err := resolveGoModule("Cladamos/solcl")
	if err != nil || module != "github.com/cladamos/solcl" || repo != "solcl" {
		t.Errorf("got (%q, %q, %v)", module, repo, err)
	}

	module, repo, err = resolveGoModule("charm.land/glow/v3")
	if err != nil || module != "charm.land/glow/v3" || repo != "glow" {
		t.Errorf("module spec: got (%q, %q, %v)", module, repo, err)
	}
}

// fakeGo installs a goRun that answers by subcommand and records calls.
func fakeGo(t *testing.T, handler func(dir string, env []string, args []string) ([]byte, string, error)) {
	t.Helper()
	prev := goRun
	goRun = func(dir string, env []string, args ...string) ([]byte, string, error) {
		return handler(dir, env, args)
	}
	t.Cleanup(func() { goRun = prev })
}

// gobinEnv returns the effective GOBIN: the last assignment wins, as in exec.
func gobinEnv(env []string) string {
	val := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GOBIN="); ok {
			val = v
		}
	}
	return val
}

func TestDownloadGoModule(t *testing.T) {
	fakeGo(t, func(_ string, _ []string, args []string) ([]byte, string, error) {
		if strings.Join(args, " ") != "mod download -json example.com/m@latest" {
			t.Errorf("unexpected args %v", args)
		}
		return []byte(`{"Dir":"/cache/m","Version":"v1.2.3"}`), "", nil
	})
	dir, version, err := downloadGoModule("example.com/m")
	if err != nil || dir != "/cache/m" || version != "v1.2.3" {
		t.Errorf("got (%q, %q, %v)", dir, version, err)
	}
}

func TestDownloadGoModule_SurfacesToolchainError(t *testing.T) {
	const msg = "create zip: malformed file path \"a:b.webp\": invalid char ':'"
	fakeGo(t, func(string, []string, []string) ([]byte, string, error) {
		return []byte(`{"Error":"` + strings.ReplaceAll(msg, `"`, `\"`) + `"}`), "", errors.New("exit 1")
	})
	_, _, err := downloadGoModule("example.com/m")
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("err = %v, want verbatim toolchain message", err)
	}
}

func TestDownloadGoModule_StderrWhenNoJSON(t *testing.T) {
	fakeGo(t, func(string, []string, []string) ([]byte, string, error) {
		return nil, "module lookup disabled by GOPROXY=off", errors.New("exit 1")
	})
	_, _, err := downloadGoModule("example.com/m")
	if err == nil || !strings.Contains(err.Error(), "GOPROXY=off") {
		t.Errorf("err = %v, want GOPROXY=off message", err)
	}
}

func TestListGoMainPackages(t *testing.T) {
	fakeGo(t, func(dir string, _ []string, args []string) ([]byte, string, error) {
		if dir != "/cache/m" || args[0] != "list" || args[1] != "-e" {
			t.Errorf("unexpected call dir=%q args=%v", dir, args)
		}
		return []byte("example.com/m/cmd/a\n\n\nexample.com/m/cmd/b\n\n"), "", nil
	})
	got, err := listGoMainPackages("/cache/m")
	if err != nil || strings.Join(got, ",") != "example.com/m/cmd/a,example.com/m/cmd/b" {
		t.Errorf("got (%v, %v)", got, err)
	}
}

// goPathSandbox points GOPATH at a temp dir so GoBinDir resolves inside it.
func goPathSandbox(t *testing.T) (binDir string) {
	t.Helper()
	gopath := t.TempDir()
	t.Setenv("GOPATH", gopath)
	return filepath.Join(gopath, "bin")
}

func TestGoInstall_EndToEndWithFakeToolchain(t *testing.T) {
	binDir := goPathSandbox(t)
	rawServer(t, map[string]string{
		"/u/cyberspace-tui-go/HEAD/go.mod": "module github.com/u/cyberspace-tui-go\n",
	})
	var installed string
	fakeGo(t, func(_ string, env []string, args []string) ([]byte, string, error) {
		switch args[0] {
		case "mod":
			return []byte(`{"Dir":"/cache","Version":"v0.3.0"}`), "", nil
		case "list":
			return []byte("github.com/u/cyberspace-tui-go/cmd\n"), "", nil
		case "install":
			installed = args[1]
			if err := os.WriteFile(filepath.Join(gobinEnv(env), "cmd"), []byte("bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			return nil, "", nil
		}
		t.Fatalf("unexpected go %v", args)
		return nil, "", nil
	})

	bin, pkg, err := GoInstall("u/cyberspace-tui-go")
	if err != nil {
		t.Fatalf("GoInstall: %v", err)
	}
	if bin != "cyberspace-tui-go" || pkg != "github.com/u/cyberspace-tui-go/cmd" {
		t.Errorf("got (%q, %q)", bin, pkg)
	}
	if installed != "github.com/u/cyberspace-tui-go/cmd@v0.3.0" {
		t.Errorf("install target = %q, want pinned version", installed)
	}
	if _, err := os.Stat(filepath.Join(binDir, "cyberspace-tui-go")); err != nil {
		t.Errorf("binary not moved into go bin: %v", err)
	}
	entries, _ := os.ReadDir(binDir)
	if len(entries) != 1 {
		t.Errorf("scratch dir left behind: %v", entries)
	}
}

func TestGoInstall_AmbiguousInstallsNothing(t *testing.T) {
	binDir := goPathSandbox(t)
	rawServer(t, map[string]string{"/u/tool/HEAD/go.mod": "module github.com/u/tool\n"})
	fakeGo(t, func(_ string, _ []string, args []string) ([]byte, string, error) {
		switch args[0] {
		case "mod":
			return []byte(`{"Dir":"/cache","Version":"v1.0.0"}`), "", nil
		case "list":
			return []byte("github.com/u/tool/cmd/a\ngithub.com/u/tool/cmd/b\n"), "", nil
		}
		t.Fatalf("go %v must not run", args)
		return nil, "", nil
	})

	_, _, err := GoInstall("u/tool")
	var ambig *AmbiguousGoPackageError
	if !errors.As(err, &ambig) {
		t.Fatalf("err = %v, want AmbiguousGoPackageError", err)
	}
	if _, statErr := os.Stat(binDir); statErr == nil {
		entries, _ := os.ReadDir(binDir)
		if len(entries) != 0 {
			t.Errorf("nothing should be installed, found %v", entries)
		}
	}
}

func TestGoUpdate_KeepsBinaryName(t *testing.T) {
	binDir := goPathSandbox(t)
	fakeGo(t, func(_ string, env []string, args []string) ([]byte, string, error) {
		if args[1] != "github.com/u/tool/cmd@latest" {
			t.Errorf("install target = %q", args[1])
		}
		os.WriteFile(filepath.Join(gobinEnv(env), "cmd"), []byte("new"), 0o755)
		return nil, "", nil
	})
	if err := GoUpdate("tool", "github.com/u/tool/cmd"); err != nil {
		t.Fatalf("GoUpdate: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(binDir, "tool"))
	if err != nil || string(got) != "new" {
		t.Errorf("tool binary = %q, %v", got, err)
	}
	if err := GoUpdate("tool", ""); err == nil {
		t.Error("empty package path must be an error")
	}
}

func TestGoInstall_ToolchainErrorVerbatim(t *testing.T) {
	goPathSandbox(t)
	rawServer(t, map[string]string{"/u/tool/HEAD/go.mod": "module github.com/u/tool\n"})
	const msg = "go: github.com/u/tool@v1.0.0 requires go >= 9.9 (running go 1.25; GOTOOLCHAIN=local)"
	fakeGo(t, func(_ string, _ []string, args []string) ([]byte, string, error) {
		switch args[0] {
		case "mod":
			return []byte(`{"Dir":"/cache","Version":"v1.0.0"}`), "", nil
		case "list":
			return []byte("github.com/u/tool\n"), "", nil
		}
		return nil, msg, errors.New("exit 1")
	})
	_, _, err := GoInstall("u/tool")
	if err == nil || !strings.Contains(err.Error(), "GOTOOLCHAIN=local") {
		t.Errorf("err = %v, want toolchain requirement", err)
	}
}

func TestGoUninstall(t *testing.T) {
	binDir := goPathSandbox(t)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(binDir, "tool")
	os.WriteFile(target, []byte("x"), 0o755)

	if err := GoUninstall("tool"); err != nil {
		t.Fatalf("GoUninstall: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("binary still present: %v", err)
	}
	if err := GoUninstall("tool"); err != nil {
		t.Errorf("missing binary must not be an error: %v", err)
	}
	if err := GoUninstall("../escape"); err == nil {
		t.Error("path traversal must be rejected")
	}
}

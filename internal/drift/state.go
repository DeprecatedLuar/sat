package drift

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DeprecatedLuar/sat/internal/config"
	"github.com/DeprecatedLuar/sat/internal/manifest"
)

// staleLockAge bounds how long a claim lock is honored after its mtime -
// longer than any plausible reconcile, so a crashed process can't wedge
// drift forever.
const staleLockAge = 5 * time.Minute

// stampPrefix is the one recognized line in the stamp file, matching the
// os-info cache's plain KEY=value convention.
const stampPrefix = "last="

func lockPath() string {
	return manifest.DriftStampPath() + ".lock"
}

// readStamp returns the last recorded reconcile time, or the zero time if
// the stamp is missing or malformed - both mean "never ran, reconcile now"
// rather than an error.
func readStamp() time.Time {
	data, err := os.ReadFile(manifest.DriftStampPath())
	if err != nil {
		return time.Time{}
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, stampPrefix) {
		return time.Time{}
	}
	unix, err := strconv.ParseInt(strings.TrimPrefix(line, stampPrefix), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

// writeStamp records now as the last reconcile time.
func writeStamp(now time.Time) error {
	dir := manifest.StateDir()
	if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
		return err
	}
	content := fmt.Sprintf("%s%d\n", stampPrefix, now.Unix())
	return os.WriteFile(manifest.DriftStampPath(), []byte(content), manifest.FilePermissions)
}

// claim atomically takes the drift lock (O_EXCL is atomic on any local
// filesystem), so two sat processes racing the same TTL window don't both
// reconcile and both write the manifest. A lock older than staleLockAge is
// assumed to belong to a crashed process and is reclaimed once.
func claim() (release func(), ok bool) {
	dir := manifest.StateDir()
	if err := os.MkdirAll(dir, manifest.DirPermissions); err != nil {
		return nil, false
	}
	path := lockPath()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, manifest.FilePermissions)
	if err != nil {
		if !os.IsExist(err) {
			return nil, false
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			os.Remove(path)
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, manifest.FilePermissions)
		}
		if err != nil {
			return nil, false
		}
	}
	f.Close()
	return func() { os.Remove(path) }, true
}

// EnsureWithin reconciles at most once per interval, claiming a lock first
// so concurrent sat invocations don't duplicate the work. interval <= 0
// disables reconciliation entirely (the config "off" case).
//
// The stamp is written BEFORE Reconcile runs, not after: it doubles as the
// herd guard (a process that claims the window makes every process
// starting behind it skip), and it caps a hanging or misbehaving package
// manager at one slow run per interval instead of every sat invocation -
// including sat --version - retrying the same failure forever.
func EnsureWithin(interval time.Duration) (int, error) {
	if interval <= 0 {
		return 0, nil
	}
	if time.Since(readStamp()) < interval {
		return 0, nil
	}

	release, ok := claim()
	if !ok {
		return 0, nil
	}
	defer release()

	// A racer may have finished between our stat and our claim.
	if time.Since(readStamp()) < interval {
		return 0, nil
	}

	if err := writeStamp(time.Now()); err != nil {
		return 0, err
	}

	return Reconcile()
}

// Ensure is the TTL-gated entrypoint selfheal calls, using the interval
// configured in config.toml (config.DefaultDriftInterval if unset or
// unparseable).
func Ensure() (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 0, err
	}
	return EnsureWithin(cfg.ResolvedDriftInterval())
}

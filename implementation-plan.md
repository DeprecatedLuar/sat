# sat restructure implementation plan

<!-- WORKFLOW IMPLEMENTATION GUIDE:
- This file contains ONLY active phases for implementation
- Each phase = one focused session, follow top-to-bottom order determined by manual commit and push by the user, so phases should be meaningful and self-contained
- Focus on actionable steps: "Update file X, add function Y"
- Avoid verbose explanations - just implement what's specified and valuable
- Success criteria must be testable
- Make sure to test implementation after conclusion of phase
- Stop implementation and call out ideas if you find better approaches during implementation
CRITICAL: When a phase is completed:
1. DELETE it entirely from this file (do not leave it here)
2. Move a concise summary to implementation-history.md
3. Do not use emojis in this file
-->

---

## Phase 3: Main binary becomes router + lib bootstrap

Transform `sat` into a pure router that checks lib status and delegates.

Steps:
- Add `SAT_LIBRARY="${XDG_DATA_HOME:-$HOME/.local/share}/sat/lib"` constant to main binary (renamed from SAT_LOCAL_LIB for clarity)
- Add `_ensure_lib()` function:
  - **Simple validation**: checks if `$SAT_LIBRARY/common.sh` exists
  - If missing, fetches lib from GitHub using **tarball API**:
    ```bash
    curl -sSL "https://github.com/DeprecatedLuar/sat/archive/refs/heads/main.tar.gz" | \
      tar xzf - --strip-components=1 -C "$SAT_LIBRARY" "sat-main/lib"
    ```
  - **Show progress**: Print "Fetching sat library from GitHub..." before fetch
  - **Fail with clear error**: If curl fails, exit with message: "Error: Failed to fetch sat library. Check network connection and firewall settings."
  - Create `$SAT_LIBRARY` directory if it doesn't exist
- Call `_ensure_lib` before every command (all commands)
- Update all `source "$SAT_LIB/..."` calls in the router to use `$SAT_LIBRARY/...`
- Add `sat pull` command:
  - Wipes `$SAT_LIBRARY/` directory
  - Calls `_ensure_lib` to re-fetch everything
  - Prints confirmation: "Library refreshed from GitHub"
  - **Library only** - does NOT regenerate os-info cache
- Remove `SAT_LIB` / `SAT_ROOT` path resolution logic that assumed local repo structure

Repo architecture:
- **This repo** (DeprecatedLuar/sat): Contains sat binary and lib/ folder - source for library tarball
- **Remote scripts** (DeprecatedLuar/the-satellite): Contains bootstrapping scripts (os_detection.sh, fetcher.sh) - referenced via SAT_BASE

Success:
- Delete `${XDG_DATA_HOME:-$HOME/.local/share}/sat/lib/`, run `sat list` → lib is fetched with progress message, command works
- Run `sat pull` → lib is refreshed, os-info unchanged
- Symlink `${XDG_DATA_HOME:-$HOME/.local/share}/sat/lib` → repo `lib/` for dev workflow, confirm commands use local files
- Test with no network after lib fetched → commands work (cached)
- Test with no network and no lib → clear error message displayed

---

## Phase 4: Fix tmux rcfile paths

Update shell.sh to use `$SAT_LIBRARY` paths in the generated rcfile.

Steps:
- In `lib/commands/shell.sh`, update the rcfile heredoc:
  - Pass `SAT_LIBRARY` into the rcfile vars block so it's available inside tmux
  - Replace `source "$SAT_LIB/common.sh"` → `source "$SAT_LIBRARY/common.sh"`
  - Replace `source "$SAT_LIB/install.sh"` → `source "$SAT_LIBRARY/commands/install.sh"`
  - Remove old `SAT_LIB=` line from rcfile vars block
- Verify `SAT_SESSION` and `SAT_MANIFEST_TARGET` env vars are still passed in correctly

Success: run `sat shell cowsay:sys`, tools install, exit cleanly, cleanup runs. Verify with `sat list` no session tools remain.
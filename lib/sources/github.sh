#!/usr/bin/env bash
# GitHub - Self-contained source module for GitHub-based installs

# Source appimage module for delegation
source "$SAT_LIB/sources/appimage.sh"

# Temp file for subshell communication
_GH_RESULT_FILE="/tmp/sat-gh-result-$$"

# In-memory cache for release binaries
declare -A _RELEASE_CACHE

# Shared GitHub API wrapper
_gh_api() {
    local endpoint="$1"
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        gh api "$endpoint" 2>/dev/null
    else
        curl -sS "https://api.github.com/$endpoint"
    fi
}

# Check if API response is a rate limit error
_gh_check_rate_limit() {
    local response="$1"
    if echo "$response" | jq -e '.message' 2>/dev/null | grep -qi "rate limit"; then
        echo "Error: GitHub API rate limit exceeded" >&2
        echo "Tip: Authenticate with 'gh auth login' for higher limits (60/hour → 5000/hour)" >&2
        return 1
    fi
    return 0
}

# Fetch repo tree structure (tries main, then master)
_fetch_tree() {
    local repo="$1"
    local response tree

    # Try main branch
    response=$(_gh_api "repos/$repo/git/trees/main?recursive=1")
    _gh_check_rate_limit "$response" || return 1
    tree=$(echo "$response" | jq -r '.tree[].path' 2>/dev/null)

    # Fallback to master if main failed
    if [[ -z "$tree" || "$tree" == "null" ]]; then
        response=$(_gh_api "repos/$repo/git/trees/master?recursive=1")
        _gh_check_rate_limit "$response" || return 1
        tree=$(echo "$response" | jq -r '.tree[].path' 2>/dev/null)
    fi

    echo "$tree"
}

# Get binary names from latest release (cached)
_get_release_binaries() {
    local repo="$1"

    # Check in-memory cache
    [[ -n "${_RELEASE_CACHE[$repo]}" ]] && echo "${_RELEASE_CACHE[$repo]}" && return 0

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   fetching release binaries for $repo" >&2

    # Fetch latest release assets
    local response=$(_gh_api "repos/$repo/releases/latest")
    _gh_check_rate_limit "$response" || return 1

    local assets=$(echo "$response" | jq -r '.assets[]?.name' 2>/dev/null)
    [[ -z "$assets" ]] && return 1

    # Extract binary names (strip platform suffixes and extensions)
    # Examples: saul-linux-amd64 → saul, tool-darwin-arm64.exe → tool
    local bins=$(echo "$assets" | \
        grep -vE '\.(zip|tar\.gz|sig|sha256|txt|md)$' | \
        sed -E 's/-(linux|darwin|windows|amd64|arm64|x86_64|aarch64|i686|armv7l).*$//' | \
        sort -u | \
        tr '\n' ' ')

    # Cache result
    _RELEASE_CACHE[$repo]="$bins"
    echo "$bins"
}

# Write result to temp file (for subshell communication)
_gh_set_result() {
    local bin="$1" src="$2"
    echo "BIN=$bin" > "$_GH_RESULT_FILE"
    echo "SRC=$src" >> "$_GH_RESULT_FILE"
}

# Read result from temp file
_gh_get_result() {
    [[ -f "$_GH_RESULT_FILE" ]] || return 1
    source "$_GH_RESULT_FILE"
    _GH_INSTALLED_BIN="$BIN"
    _GH_INSTALL_SOURCE="$SRC"
    rm -f "$_GH_RESULT_FILE"
}

# =============================================================================
# SEARCH
# =============================================================================

# Search GitHub repositories (GraphQL for version data)
search_github() {
    local query="$1"
    local limit="${2:-10}"

    local graphql_query=$(cat <<EOF
{
  search(query: "$query", type: REPOSITORY, first: $limit) {
    nodes {
      ... on Repository {
        nameWithOwner
        description
        stargazerCount
        primaryLanguage {
          name
        }
        latestRelease {
          tagName
        }
      }
    }
  }
}
EOF
)

    local response
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        response=$(gh api graphql -f query="$graphql_query" 2>/dev/null)
    else
        # Curl fallback (with optional GITHUB_TOKEN support)
        local auth_header=""
        [[ -n "$GITHUB_TOKEN" ]] && auth_header="Authorization: bearer $GITHUB_TOKEN"

        response=$(curl -sS "https://api.github.com/graphql" \
            ${auth_header:+-H "$auth_header"} \
            -H "Content-Type: application/json" \
            -d "{\"query\":$(echo "$graphql_query" | jq -Rs .)}" 2>/dev/null)
    fi

    _gh_check_rate_limit "$response" || return 1
    echo "$response"
}

# Find best matching GitHub repo (with filtering)
search_github_best_match() {
    local query="$1"
    local gh_data=$(search_github "$query" 10) || return 1

    echo "$gh_data" | jq -r '.data.search.nodes[]? | "\(.nameWithOwner)|\(.primaryLanguage.name // "")"' | \
        awk -F'|' -v q="$query" '
        BEGIN { IGNORECASE=1 }
        {
            full_name = $1
            lang = $2

            split(full_name, parts, "/")
            name = parts[2]

            pattern = "(^|[-_@/.])" q "($|[-_@/.])"
            if (name ~ pattern) {
                print full_name "|" lang
                exit
            }
        }'
}

# =============================================================================
# INSTALLATION METHODS
# =============================================================================

# Method 1: Huber (binary releases)
_install_huber() {
    local repo_path="$1"
    local repo_name="${repo_path##*/}"

    command -v huber &>/dev/null || return 1
    _run_quiet huber install "$repo_path" || return 1

    # Get release binary names from GitHub API (cached)
    local release_bins=$(_get_release_binaries "$repo_path")
    local huber_bin=""

    # Try release names first (e.g., "saul" from better-curl-saul)
    if [[ -n "$release_bins" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   trying release names: $release_bins" >&2
        for bin in $release_bins; do
            if [[ -x "$HOME/.huber/bin/$bin" ]]; then
                huber_bin="$HOME/.huber/bin/$bin"
                [[ -n "$SAT_DEBUG" ]] && echo "[debug]   found via release name: $bin" >&2
                break
            fi
        done
    fi

    # Fallback to exact repo name
    if [[ -z "$huber_bin" && -x "$HOME/.huber/bin/$repo_name" ]]; then
        huber_bin="$HOME/.huber/bin/$repo_name"
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   found via repo name: $repo_name" >&2
    fi

    # Fallback to aggressive search (current behavior)
    if [[ -z "$huber_bin" ]]; then
        [[ -n "$SAT_DEBUG" ]] && echo "[debug]   trying aggressive search" >&2
        local search_pattern="${repo_name//-/[-_]?}"  # Allow - or _ or nothing
        huber_bin=$(find "$HOME/.huber/bin/" -maxdepth 1 -type l -iname "*${repo_name%%-*}*" 2>/dev/null | head -1)
        [[ -z "$huber_bin" || ! -x "$huber_bin" ]] && return 1
    fi

    local bin_name=$(basename "$huber_bin")
    mkdir -p "$HOME/.local/bin"
    ln -sf "$huber_bin" "$HOME/.local/bin/$bin_name"
    _gh_set_result "$bin_name" "gh:$repo_path"
    return 0
}

# Method 2: Go (build from GitHub)
_install_go() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v go &>/dev/null || return 1
    echo "$tree" | grep -q '^go.mod$' || return 1

    local go_bin go_path go_subdir=""

    go_bin=$(echo "$tree" | grep -oP '^cmd/\K[^/]+(?=/main\.go$)' | head -1)
    [[ -n "$go_bin" ]] && go_subdir="cmd"
    [[ -z "$go_bin" ]] && echo "$tree" | grep -q "^${repo_name}/main\.go$" && go_bin="$repo_name"

    if [[ -n "$go_bin" ]]; then
        go_path="github.com/$repo_path/${go_subdir:+$go_subdir/}$go_bin@latest"
    elif echo "$tree" | grep -q '^main.go$'; then
        go_path="github.com/$repo_path@latest"
    else
        return 1
    fi

    _run_quiet go install "$go_path" || return 1
    _gh_set_result "${go_bin:-$repo_name}" "go:github.com/$repo_path"
    return 0
}

# Method 3: Python (build from GitHub)
_install_python() {
    local repo_path="$1"
    local tree="$2"
    local repo_name="${repo_path##*/}"

    command -v uv &>/dev/null || return 1
    echo "$tree" | grep -qE '^(pyproject.toml|setup.py|setup.cfg)$' || return 1

    _run_quiet uv tool install "git+https://github.com/$repo_path" || return 1
    _gh_set_result "$repo_name" "uv"
    return 0
}

# Main orchestrator
install_github() {
    local input="$1"
    local method="${2:-auto}"

    [[ -n "$SAT_DEBUG" ]] && echo "[debug]   install_github: input=$input, method=$method" >&2

    local repo lang tree
    rm -f "$_GH_RESULT_FILE"

    if [[ "$input" == */* ]]; then
        repo="$input"
    else
        local result=$(search_github_best_match "$input")
        [[ -z "$result" ]] && return 1
        repo="${result%|*}"
        lang="${result#*|}"
    fi

    tree=$(_fetch_tree "$repo")
    [[ -z "$tree" || "$tree" == "null" ]] && return 1

    case "$method" in
        auto)
            _install_huber "$repo" && return 0

            if install_appimage "$repo"; then
                local app_name=$(install_appimage "$repo" 2>&1 | tail -1)
                _gh_set_result "$app_name" "appimage:$repo"
                return 0
            fi

            case "$lang" in
                Go)     _install_go "$repo" "$tree" && return 0 ;;
                Python) _install_python "$repo" "$tree" && return 0 ;;
                "")     _install_go "$repo" "$tree" && return 0
                        _install_python "$repo" "$tree" && return 0 ;;
            esac

            return 1
            ;;
        release)
            command -v huber &>/dev/null || { echo "huber required" >&2; return 1; }
            _install_huber "$repo"
            ;;
        appimage)
            if install_appimage "$repo"; then
                local app_name=$(install_appimage "$repo" 2>&1 | tail -1)
                _gh_set_result "$app_name" "appimage:$repo"
                return 0
            fi
            return 1
            ;;
        *)
            return 1
            ;;
    esac
}

# Uninstall
uninstall_github() {
    local pkg="$1"
    local source="$2"

    # If source has repo metadata (gh:owner/repo), try huber first
    if [[ "$source" == gh:*/* ]]; then
        local repo="${source#gh:}"
        if command -v huber &>/dev/null; then
            huber uninstall "$repo" 2>/dev/null && return 0
        fi
    fi

    # Fallback: manual removal (for scripts or if huber unavailable)
    rm -f "$HOME/.local/bin/$pkg" "$HOME/bin/$pkg" 2>/dev/null
    # Also remove huber symlink if it exists
    rm -f "$HOME/.huber/bin/$pkg" 2>/dev/null
}

# Update GitHub package (huber-managed only)
update_github() {
    local tool="$1"
    local repo="$2"  # Format: owner/repo from gh:* manifest

    command -v huber &>/dev/null || return 1
    _run_quiet huber update "$repo"
}

# Query latest version from GitHub releases (before install)
query_latest_version_github() {
    local repo="$1"  # Format: owner/repo
    github_api_get "repos/$repo/releases/latest" | jq -r '.tag_name // empty'
}

# Get installed version from GitHub release (via huber or GitHub API)
get_version_from_github() {
    local repo="$1"  # Format: owner/repo

    # Try huber first (if installed via huber)
    if command -v huber &>/dev/null; then
        local info=$(huber info "$repo" 2>/dev/null)
        if [[ -n "$info" ]]; then
            echo "$info" | grep -oP 'Version:\s*\K\S+'
            return 0
        fi
    fi

    # Fallback: query GitHub API for latest release tag
    query_latest_version_github "$repo"
}

# Check if GitHub package is outdated (huber-managed)
check_outdated_github() {
    local tool="$1"
    local repo="$2"  # Format: owner/repo from gh:* manifest

    command -v huber &>/dev/null || return 1

    # Try manifest first
    local current=$(get_source_version "$(manifest_get "$tool")")

    # Fall back to huber dryrun
    if [[ -z "$current" ]]; then
        local dryrun=$(huber update --dryrun 2>&1)
        [[ -z "$dryrun" ]] && return 1

        local line=$(echo "$dryrun" | grep -i "Updating package $repo from")
        [[ -z "$line" ]] && return 1

        current=$(echo "$line" | grep -oP 'from \K\S+')
    fi
    [[ -z "$current" ]] && return 1

    # Get latest from GitHub
    local latest=$(github_api_get "repos/$repo/releases/latest" | jq -r '.tag_name // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return 1
    echo "$current $latest"
}

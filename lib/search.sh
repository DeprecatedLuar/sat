#!/usr/bin/env bash
# sat search - find packages across sources (parallel, API-based)

# =============================================================================
# INDIVIDUAL SEARCH FUNCTIONS
# =============================================================================

# Search GitHub repositories
# Returns: JSON with items array
search_github() {
    local query="$1"
    local limit="${2:-10}"

    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        gh api "search/repositories?q=$query+in:name&per_page=$limit" 2>/dev/null
    else
        curl -sS "https://api.github.com/search/repositories?q=$query+in:name&per_page=$limit" 2>/dev/null
    fi
}

# Search system package manager (apt/pacman/apk/dnf)
search_system() {
    local query="$1"
    local mgr=$(get_pkg_manager)
    [[ -z "$mgr" ]] && return 1

    case "$mgr" in
        apt)    apt-cache search "$query" 2>/dev/null | head -30 ;;
        pacman) pacman -Ss "$query" 2>/dev/null | grep -A1 "^[^ ]" | head -30 ;;
        apk)    apk search "$query" 2>/dev/null | head -30 ;;
        dnf)    dnf search "$query" 2>/dev/null | grep -v "^=" | head -30 ;;
    esac
}

# Search crates.io (Rust)
search_cargo() {
    local query="$1"
    curl -sS "https://crates.io/api/v1/crates?q=$query&per_page=10" 2>/dev/null | \
        jq -r '.crates[]? | "\(.name) \(.max_version) - \(.description // "" | split("\n")[0])"' 2>/dev/null
}

# Search npmjs (Node)
search_npm() {
    local query="$1"
    curl -sS "https://registry.npmjs.org/-/v1/search?text=$query&size=10" 2>/dev/null | \
        jq -r '.objects[]? | "\(.package.name) \(.package.version) - \(.package.description // "" | split("\n")[0])"' 2>/dev/null
}

# Search Homebrew
search_brew() {
    local query="$1"
    local info=$(curl -sS "https://formulae.brew.sh/api/formula/$query.json" 2>/dev/null)
    if echo "$info" | jq -e '.name' &>/dev/null; then
        echo "$info" | jq -r '"\(.name) \(.versions.stable) - \(.desc // "" | split("\n")[0])"' 2>/dev/null
    fi
}

# Search NixOS packages
search_nix() {
    local query="$1"
    curl -sS "https://aWVSALXpZv:X8gPHnzL52wFEekuxsfQ9cSh@nixos-search-7-1733963800.us-east-1.bonsaisearch.net/latest-*/_search" \
        -H "Content-Type: application/json" \
        -d "{\"query\":{\"bool\":{\"should\":[{\"term\":{\"package_attr_name\":{\"value\":\"$query\",\"boost\":10}}},{\"wildcard\":{\"package_attr_name\":\"*$query*\"}}],\"minimum_should_match\":1}},\"size\":20,\"_source\":[\"package_attr_name\",\"package_pversion\",\"package_description\"]}" 2>/dev/null | \
        jq -r '.hits.hits[]._source | "\(.package_attr_name) \(.package_pversion) - \(.package_description // "no description")"' 2>/dev/null | \
        awk -F' ' '!seen[$1]++' | head -10
}

# Search PyPI
search_pypi() {
    local query="$1"
    local info=$(curl -sS "https://pypi.org/pypi/$query/json" 2>/dev/null)
    if echo "$info" | jq -e '.info' &>/dev/null; then
        echo "$info" | jq -r '"\(.info.name) \(.info.version) - \(.info.summary // "" | split("\n")[0])"' 2>/dev/null
    fi
}

# =============================================================================
# DISPLAY HELPERS
# =============================================================================

colorize_result() {
    local name_color="$1"
    LC_ALL=C awk -v nc="$name_color" '{
        if (match($0, / - /)) {
            pre = substr($0, 1, RSTART-1)
            desc = substr($0, RSTART)
            split(pre, parts, " ")
            name = parts[1]
            version = (length(parts) > 1) ? parts[2] : ""
            printf "%s%s\033[0m %s\033[2m%s\033[0m\n", nc, name, version, desc
        } else {
            split($0, parts, " ")
            name = parts[1]
            version = (length(parts) > 1) ? " " parts[2] : ""
            printf "%s%s\033[0m%s\n", nc, name, version
        }
    }'
}

filter_relevant() {
    local q="$1"
    awk -v query="$q" '
    BEGIN { IGNORECASE=1 }
    {
        name = $1
        pattern = "(^|[-_@/])" query "($|[-_@/])"
        if (name ~ pattern) print
    }'
}

# =============================================================================
# SOURCE MAPPING
# =============================================================================

# Map source aliases to search functions
_search_source_func() {
    case "$1" in
        gh|github|repo)     echo "github" ;;
        sys|system|apt)     echo "system" ;;
        rs|rust|cargo)      echo "cargo" ;;
        js|node|npm)        echo "npm" ;;
        py|python|uv|pypi)  echo "pypi" ;;
        brew|homebrew)      echo "brew" ;;
        nix)                echo "nix" ;;
        *)                  echo "" ;;
    esac
}

# =============================================================================
# MAIN SEARCH FUNCTION
# =============================================================================

sat_search() {
    local QUERY=""
    local SOURCE=""
    local NO_WRAP=true
    local FILTER=true

    for arg in "$@"; do
        case "$arg" in
            --wrap) NO_WRAP=false ;;
            --all)  FILTER=false ;;
            *)      QUERY="$arg" ;;
        esac
    done

    [[ -z "$QUERY" ]] && { echo "Usage: sat search <program>[:source] [--wrap] [--all]"; return 1; }

    # Parse source specifier (query:source)
    if [[ "$QUERY" == *:* ]]; then
        SOURCE="${QUERY##*:}"
        QUERY="${QUERY%:*}"
    fi

    local TERM_WIDTH=${COLUMNS:-$(tput cols 2>/dev/null || echo 80)}
    local CONTENT_WIDTH=$((TERM_WIDTH - 4))

    # Header
    local header="──[${QUERY^^}]"
    local padding=$((TERM_WIDTH - ${#header}))
    printf "%s%s\n\n" "$header" "$(printf '─%.0s' $(seq 1 $padding))"

    # Single source search
    if [[ -n "$SOURCE" ]]; then
        local func=$(_search_source_func "$SOURCE")
        [[ -z "$func" ]] && { echo "Unknown source: $SOURCE"; return 1; }

        local results
        if [[ "$func" == "github" ]]; then
            results=$(search_github "$QUERY" 10 | jq -r '.items[]? | "\(.full_name) (\(.language // "unknown" | ascii_downcase)) *\(.stargazers_count) - \(.description // "" | split("\n")[0] | .[0:50])"' 2>/dev/null)
        else
            results=$(search_$func "$QUERY")
        fi

        if [[ -n "$results" ]]; then
            local color=$(source_color "$func")
            printf "${color}%s:${C_RESET}\n" "$func"
            if $FILTER; then
                echo "$results" | filter_relevant "$QUERY" | cut -c1-"$CONTENT_WIDTH" | colorize_result "$color" | sed 's/^/  /'
            else
                echo "$results" | cut -c1-"$CONTENT_WIDTH" | colorize_result "$color" | sed 's/^/  /'
            fi
        else
            echo "No results found in $func"
        fi
        return 0
    fi

    # Multi-source parallel search
    local tmpdir=$(mktemp -d)

    search_system "$QUERY" > "$tmpdir/system" 2>/dev/null &
    search_cargo "$QUERY" > "$tmpdir/rust" 2>/dev/null &
    search_npm "$QUERY" > "$tmpdir/node" 2>/dev/null &
    search_github "$QUERY" 10 > "$tmpdir/github_raw" 2>/dev/null &
    search_brew "$QUERY" > "$tmpdir/brew" 2>/dev/null &
    search_nix "$QUERY" > "$tmpdir/nix" 2>/dev/null &

    wait

    # Process GitHub for display + extract Python repos for PyPI lookup
    jq -r '.items[]? | "\(.full_name) (\(.language // "unknown" | ascii_downcase)) *\(.stargazers_count) - \(.description // "" | split("\n")[0] | .[0:50])"' \
        < "$tmpdir/github_raw" 2>/dev/null > "$tmpdir/github"

    local python_repos=$(jq -r '.items[]? | select(.language == "Python") | .name' < "$tmpdir/github_raw" 2>/dev/null | head -5)
    touch "$tmpdir/python"

    search_pypi "$QUERY" >> "$tmpdir/python" &

    for repo in $python_repos; do
        search_pypi "$repo" >> "$tmpdir/python" &
    done

    wait
    sort -u "$tmpdir/python" -o "$tmpdir/python" 2>/dev/null

    # Display results
    declare -A color_map=([system]="apt" [rust]="cargo" [python]="uv" [node]="npm" [github]="repo" [brew]="brew" [nix]="nix")
    declare -A light_map=([system]="$C_SYSTEM_L" [rust]="$C_RUST_L" [python]="$C_PYTHON_L" [node]="$C_NODE_L" [github]="$C_REPO_L" [brew]="$C_BREW_L" [nix]="$C_NIX_L")

    for source in system brew nix rust python node github; do
        if [[ -s "$tmpdir/$source" ]]; then
            local filtered="$tmpdir/${source}_filtered"
            if $FILTER; then
                filter_relevant "$QUERY" < "$tmpdir/$source" > "$filtered"
            else
                cp "$tmpdir/$source" "$filtered"
            fi

            [[ ! -s "$filtered" ]] && continue

            local color=$(source_color "${color_map[$source]}")
            local light="${light_map[$source]}"
            printf "${color}%s:${C_RESET}\n" "$source"
            if $NO_WRAP; then
                cut -c1-"$CONTENT_WIDTH" "$filtered" | colorize_result "$light" | sed 's/^/  /'
            else
                colorize_result "$light" < "$filtered" | sed 's/^/  /'
            fi
            echo
        fi
    done

    rm -rf "$tmpdir"
}

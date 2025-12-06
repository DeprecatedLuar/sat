#!/usr/bin/env bash
# sat search - find packages across sources (parallel, API-based)

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

sat_search() {
    local QUERY=""
    local NO_WRAP=true
    local FILTER=true

    for arg in "$@"; do
        case "$arg" in
            --wrap) NO_WRAP=false ;;
            --all)  FILTER=false ;;
            *)      QUERY="$arg" ;;
        esac
    done

    [[ -z "$QUERY" ]] && { echo "Usage: sat search <program> [--wrap] [--all]"; return 1; }

    local tmpdir=$(mktemp -d)
    local PKG_MGR=$(get_pkg_manager)
    local TERM_WIDTH=${COLUMNS:-$(tput cols 2>/dev/null || echo 80)}
    local CONTENT_WIDTH=$((TERM_WIDTH - 4))

    # Phase 1: Parallel searches
    (
        if [[ -n "$PKG_MGR" ]]; then
            case "$PKG_MGR" in
                apt) apt-cache search "$QUERY" 2>/dev/null | head -30 ;;
                pacman) pacman -Ss "$QUERY" 2>/dev/null | grep -A1 "^[^ ]" | head -30 ;;
                apk) apk search "$QUERY" 2>/dev/null | head -30 ;;
                dnf) dnf search "$QUERY" 2>/dev/null | grep -v "^=" | head -30 ;;
            esac
        fi
    ) > "$tmpdir/system" 2>/dev/null &

    (
        curl -sS "https://crates.io/api/v1/crates?q=$QUERY&per_page=5" 2>/dev/null | \
            jq -r '.crates[]? | "\(.name) \(.max_version) - \(.description // "" | split("\n")[0])"' 2>/dev/null
    ) > "$tmpdir/rust" 2>/dev/null &

    (
        curl -sS "https://registry.npmjs.org/-/v1/search?text=$QUERY&size=5" 2>/dev/null | \
            jq -r '.objects[]? | "\(.package.name) \(.package.version) - \(.package.description // "" | split("\n")[0])"' 2>/dev/null
    ) > "$tmpdir/node" 2>/dev/null &

    (
        if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
            gh api "search/repositories?q=$QUERY+in:name&per_page=10" 2>/dev/null
        else
            curl -sS "https://api.github.com/search/repositories?q=$QUERY+in:name&per_page=10" 2>/dev/null
        fi
    ) > "$tmpdir/github_raw" 2>/dev/null &

    (
        local info=$(curl -sS "https://formulae.brew.sh/api/formula/$QUERY.json" 2>/dev/null)
        if echo "$info" | jq -e '.name' &>/dev/null; then
            echo "$info" | jq -r '"\(.name) \(.versions.stable) - \(.desc // "" | split("\n")[0])"' 2>/dev/null
        fi
    ) > "$tmpdir/brew" 2>/dev/null &

    # Nix: use NixOS Elasticsearch API (public read-only credentials)
    (
        curl -sS "https://aWVSALXpZv:X8gPHnzL52wFEekuxsfQ9cSh@nixos-search-7-1733963800.us-east-1.bonsaisearch.net/latest-*/_search" \
            -H "Content-Type: application/json" \
            -d "{\"query\":{\"bool\":{\"should\":[{\"term\":{\"package_attr_name\":{\"value\":\"$QUERY\",\"boost\":10}}},{\"wildcard\":{\"package_attr_name\":\"*$QUERY*\"}}],\"minimum_should_match\":1}},\"size\":20,\"_source\":[\"package_attr_name\",\"package_pversion\",\"package_description\"]}" 2>/dev/null | \
            jq -r '.hits.hits[]._source | "\(.package_attr_name) \(.package_pversion) - \(.package_description // "no description")"' 2>/dev/null | \
            awk -F' ' '!seen[$1]++' | head -10
    ) > "$tmpdir/nix" 2>/dev/null &

    wait

    # Phase 2: Process GitHub results for Python packages
    jq -r '.items[]? | "\(.full_name) (\(.language // "unknown" | ascii_downcase)) *\(.stargazers_count) - \(.description // "" | split("\n")[0] | .[0:50])"' \
        < "$tmpdir/github_raw" 2>/dev/null > "$tmpdir/github"

    local python_repos=$(jq -r '.items[]? | select(.language == "Python") | .name' < "$tmpdir/github_raw" 2>/dev/null | head -5)
    touch "$tmpdir/python"

    (
        local info=$(curl -sS "https://pypi.org/pypi/$QUERY/json" 2>/dev/null)
        if echo "$info" | jq -e '.info' &>/dev/null; then
            echo "$info" | jq -r '"\(.info.name) \(.info.version) - \(.info.summary // "" | split("\n")[0])"' 2>/dev/null
        fi
    ) >> "$tmpdir/python" &

    for repo in $python_repos; do
        (
            local info=$(curl -sS "https://pypi.org/pypi/$repo/json" 2>/dev/null)
            if echo "$info" | jq -e '.info' &>/dev/null; then
                echo "$info" | jq -r '"\(.info.name) \(.info.version) - \(.info.summary // "" | split("\n")[0])"' 2>/dev/null
            fi
        ) >> "$tmpdir/python" &
    done

    wait
    sort -u "$tmpdir/python" -o "$tmpdir/python" 2>/dev/null

    # Display results
    local header="──[${QUERY^^}]"
    local padding=$((TERM_WIDTH - ${#header}))
    printf "%s%s\n\n" "$header" "$(printf '─%.0s' $(seq 1 $padding))"

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

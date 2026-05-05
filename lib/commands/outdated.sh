#!/usr/bin/env bash
# outdated.sh - Check for available package updates

# Each checker writes "tool current latest" lines to a temp file

_outdated_brew() {
    local out="$1"; shift
    local tools=("$@")
    command -v brew &>/dev/null || return

    local outdated
    outdated=$(brew outdated --verbose 2>/dev/null)
    [[ -z "$outdated" ]] && return

    for tool in "${tools[@]}"; do
        local line
        line=$(echo "$outdated" | grep "^$tool (")
        [[ -z "$line" ]] && continue
        # format: "tool (current) < latest"
        local current latest
        current=$(echo "$line" | grep -oP '\(\K[^)]+')
        latest=$(echo "$line" | grep -oP '< \K\S+')
        [[ -n "$current" && -n "$latest" ]] && echo "brew $tool $current $latest" >> "$out"
    done
}

_outdated_gh() {
    local out="$1"; shift
    local tools=("$@")
    command -v huber &>/dev/null || return

    local dryrun
    dryrun=$(huber update --dryrun 2>&1)
    [[ -z "$dryrun" ]] && return

    for tool in "${tools[@]}"; do
        local repo="${tool#gh:}"
        local name="${repo##*/}"
        local line
        line=$(echo "$dryrun" | grep -i "package $name from")
        [[ -z "$line" ]] && continue
        local from to
        from=$(echo "$line" | grep -oP 'from \K\S+')
        to=$(echo "$line" | grep -oP ' to \K\S+')
        [[ -n "$from" && -n "$to" ]] && echo "gh $name $from $to" >> "$out"
    done
}

_outdated_npm() {
    local out="$1"; shift
    local tools=("$@")
    command -v npm &>/dev/null || return

    local json
    json=$(npm outdated -g --json 2>/dev/null)
    [[ -z "$json" ]] && return

    for tool in "${tools[@]}"; do
        local current latest
        current=$(echo "$json" | jq -r --arg t "$tool" '.[$t].current // empty')
        latest=$(echo "$json" | jq -r --arg t "$tool" '.[$t].latest // empty')
        [[ -z "$current" || -z "$latest" || "$current" == "$latest" ]] && continue
        echo "npm $tool $current $latest" >> "$out"
    done
}

_outdated_cargo() {
    local out="$1" tool="$2"
    command -v cargo &>/dev/null || return
    local current
    current=$(cargo install --list 2>/dev/null | grep -oP "^${tool} v\K[^ :]+")
    [[ -z "$current" ]] && return
    local latest
    latest=$(curl -s "https://crates.io/api/v1/crates/$tool" | jq -r '.crate.newest_version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return
    echo "cargo $tool $current $latest" >> "$out"
}

_outdated_uv() {
    local out="$1" tool="$2"
    command -v uv &>/dev/null || return
    local current
    current=$(uv tool list 2>/dev/null | grep -oP "^${tool} \K\S+")
    [[ -z "$current" ]] && return
    local latest
    latest=$(curl -s "https://pypi.org/pypi/$tool/json" | jq -r '.info.version // empty')
    [[ -z "$latest" || "$current" == "$latest" ]] && return
    echo "uv $tool $current $latest" >> "$out"
}

_outdated_nix() {
    local out="$1" tool="$2"
    command -v nix-env &>/dev/null || return
    local current latest
    current=$(nix-env -q "$tool" 2>/dev/null | grep -oP "\d[\d.]+")
    [[ -z "$current" ]] && return
    latest=$(nix-env -qaA "nixpkgs.$tool" 2>/dev/null | grep -oP "\d[\d.]+")
    [[ -z "$latest" || "$current" == "$latest" ]] && return
    echo "nix $tool $current $latest" >> "$out"
}

_outdated_apt() {
    local out="$1" tool="$2"
    local line
    line=$(apt list --upgradable 2>/dev/null | grep "^$tool/")
    [[ -z "$line" ]] && return
    local current latest
    current=$(echo "$line" | grep -oP 'upgradable from: \K[^]]+' | tr -d ']')
    latest=$(echo "$line" | awk '{print $2}')
    [[ -n "$current" && -n "$latest" ]] && echo "apt $tool $current $latest" >> "$out"
}

_outdated_system() {
    local out="$1" tool="$2" source="$3"
    case "$source" in
        apt)   _outdated_apt "$out" "$tool" ;;
        # pacman/apk/dnf don't have reliable per-tool outdated checks without sudo
    esac
}

sat_outdated() {
    [[ ! -s "$SAT_MANIFEST" ]] && { echo "No packages tracked."; return; }

    local tmp_out
    tmp_out=$(mktemp)

    # Collect tools by source
    local brew_tools=() gh_tools=() npm_tools=()
    local pids=()

    while IFS='=' read -r tool source; do
        [[ -z "$tool" || "$tool" == \#* ]] && continue
        case "$source" in
            brew)        brew_tools+=("$tool") ;;
            gh:*)        gh_tools+=("$source") ;;
            npm)         npm_tools+=("$tool") ;;
            cargo)       ( _outdated_cargo "$tmp_out" "$tool" ) & pids+=($!) ;;
            uv)          ( _outdated_uv    "$tmp_out" "$tool" ) & pids+=($!) ;;
            nix)         ( _outdated_nix   "$tmp_out" "$tool" ) & pids+=($!) ;;
            apt|pacman|apk|dnf) ( _outdated_system "$tmp_out" "$tool" "$source" ) & pids+=($!) ;;
            go:*|sat|repo:*) ;;  # can't check
        esac
    done < "$SAT_MANIFEST"

    # Batch checks (one call per source)
    [[ ${#brew_tools[@]} -gt 0 ]] && ( _outdated_brew "$tmp_out" "${brew_tools[@]}" ) & pids+=($!)
    [[ ${#gh_tools[@]}   -gt 0 ]] && ( _outdated_gh   "$tmp_out" "${gh_tools[@]}"   ) & pids+=($!)
    [[ ${#npm_tools[@]}  -gt 0 ]] && ( _outdated_npm  "$tmp_out" "${npm_tools[@]}"  ) & pids+=($!)

    # Wait for all checks
    for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null; done

    if [[ ! -s "$tmp_out" ]]; then
        rm -f "$tmp_out"
        echo "All packages up to date."
        return
    fi

    # Display grouped by source
    local prev_src=""
    while IFS=' ' read -r source tool current latest; do
        if [[ "$source" != "$prev_src" ]]; then
            [[ -n "$prev_src" ]] && echo ""
            local color
            color=$(source_color "$source")
            printf "${color}%s${C_RESET}:\n" "$source"
            prev_src="$source"
        fi
        local light
        light=$(source_light "$source")
        printf "  ${light}%-20s${C_RESET}  %s → %s\n" "$tool" "$current" "$latest"
    done < <(sort "$tmp_out")

    rm -f "$tmp_out"
}

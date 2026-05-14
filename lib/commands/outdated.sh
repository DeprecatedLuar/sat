#!/usr/bin/env bash
# outdated.sh - Check for available package updates

# Source all source modules
source "$SAT_LIB/sources/brew.sh"
source "$SAT_LIB/sources/cargo.sh"
source "$SAT_LIB/sources/flatpak.sh"
source "$SAT_LIB/sources/go.sh"
source "$SAT_LIB/sources/nix.sh"
source "$SAT_LIB/sources/npm.sh"
source "$SAT_LIB/sources/uv.sh"
source "$SAT_LIB/sources/system.sh"
source "$SAT_LIB/sources/github.sh"
source "$SAT_LIB/sources/appimage.sh"
source "$SAT_LIB/sources/sat.sh"

sat_outdated() {
    [[ ! -s "$SAT_MANIFEST" ]] && { echo "No packages tracked."; return; }

    local tmp_out=$(mktemp)

    # Collect packages by source type
    local -A by_source
    while IFS='=' read -r tool source; do
        [[ -z "$tool" || "$tool" == \#* ]] && continue
        local src_type="${source%%:*}"
        by_source[$src_type]+="$tool=$source "
    done < "$SAT_MANIFEST"

    # Process each source type sequentially
    for src_type in brew npm cargo nix nixos uv flatpak gh appimage go apt pacman apk dnf; do
        [[ -z "${by_source[$src_type]}" ]] && continue

        for entry in ${by_source[$src_type]}; do
            local tool="${entry%%=*}"
            local source="${entry#*=}"
            local metadata="${source#*:}"
            [[ "$metadata" == "$source" ]] && metadata=""

            local result
            case "$src_type" in
                brew)     result=$(check_outdated_brew "$tool") ;;
                npm)      result=$(check_outdated_npm "$tool") ;;
                cargo)    result=$(check_outdated_cargo "$tool") ;;
                nix|nixos) result=$(check_outdated_nix "$tool" "$src_type") ;;
                uv)       result=$(check_outdated_uv "$tool") ;;
                flatpak)  result=$(check_outdated_flatpak "$tool" "$metadata") ;;
                gh)       result=$(check_outdated_github "$tool" "$metadata") ;;
                appimage) result=$(check_outdated_appimage "$tool" "$metadata") ;;
                go)       result=$(check_outdated_go "$tool" "$metadata") ;;
                apt|pacman|apk|dnf) result=$(check_outdated_system "$tool") ;;
            esac

            [[ -n "$result" ]] && echo "$src_type $tool $result" >> "$tmp_out"
        done
    done

    # Display results
    if [[ ! -s "$tmp_out" ]]; then
        rm -f "$tmp_out"
        echo "All packages up to date."
        return
    fi

    local prev_src=""
    while IFS=' ' read -r source tool current latest; do
        if [[ "$source" != "$prev_src" ]]; then
            [[ -n "$prev_src" ]] && echo ""
            local color=$(source_color "$source")
            printf "${color}%s${C_RESET}:\n" "$source"
            prev_src="$source"
        fi
        local light=$(source_light "$source")
        printf "  ${light}%-20s${C_RESET}  %s → %s\n" "$tool" "$current" "$latest"
    done < <(sort "$tmp_out")

    rm -f "$tmp_out"
}

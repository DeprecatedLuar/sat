#!/usr/bin/env bash
# Flatpak - Self-contained source module

# Search Flathub
search_flatpak() {
    local query="$1"
    command -v flatpak &>/dev/null || return 1

    flatpak search "$query" 2>/dev/null | \
        awk -F'\t' -v q="$query" '
        BEGIN { IGNORECASE=1 }
        {
            name = $3  # Application ID
            version = $4
            desc = $2

            if (name && version && desc && !seen[name]++) {  # Deduplicate by app ID
                # Score: prefer shorter names (main apps) over plugins/addons
                components = gsub(/\./, ".", name)

                # Penalize plugins/addons heavily unless exact match
                if (tolower(name) ~ "\\.(plugin|addon|extension)\\." && tolower(name) !~ "(^|\\.)(" q ")(\\.|$)") {
                    next  # Skip plugins unless query matches component
                }

                # Boost if query matches a component exactly
                if (tolower(name) ~ "(^|\\.)(" q ")(\\.|$)") {
                    score = 1000 - components
                } else {
                    score = 500 - components
                }

                print score, name, version, "-", desc
            }
        }' | sort -rn | head -5 | cut -d' ' -f2-
}

# Install from Flathub
install_flatpak() {
    local tool="$1"
    command -v flatpak &>/dev/null || return 1

    # If it looks like an app ID (has dots), use directly
    if [[ "$tool" == *.*.* ]]; then
        flatpak install -y flathub "$tool"
        return $?
    fi

    # Search using existing function and extract app ID
    local result=$(search_flatpak "$tool" | head -1)
    [[ -z "$result" ]] && return 1

    local app_id=$(echo "$result" | awk '{print $1}')
    flatpak install -y flathub "$app_id"
}

# Uninstall flatpak package
# Handles both app names and full app IDs (org.app.Name format)
uninstall_flatpak() {
    local pkg="$1"
    local source="$2"

    # If source has metadata (flatpak:org.app.Name), use that
    if [[ "$source" == flatpak:* ]]; then
        local app_id="${source#flatpak:}"
        flatpak uninstall -y "$app_id"
    else
        # Try to find full app ID from installed apps
        local app_id=$(flatpak list --app --columns=application 2>/dev/null | grep -i "$pkg" | head -1)
        if [[ -n "$app_id" ]]; then
            flatpak uninstall -y "$app_id"
        else
            # Fallback: try package name directly
            flatpak uninstall -y "$pkg"
        fi
    fi
}

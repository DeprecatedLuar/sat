#!/usr/bin/env bash
# Flatpak - Self-contained source module

FLATPAK_WRAPPER_DIR="$HOME/.local/share/sat/bin/flatpak"
SYMLINK_DIR="$HOME/.local/bin"

# Create wrapper script for flatpak app
create_flatpak_wrapper() {
    local name="$1" app_id="$2"
    mkdir -p "$FLATPAK_WRAPPER_DIR"
    cat > "$FLATPAK_WRAPPER_DIR/$name" <<EOF
#!/usr/bin/env bash
exec flatpak run $app_id "\$@"
EOF
    chmod +x "$FLATPAK_WRAPPER_DIR/$name"
    ln -sf "$FLATPAK_WRAPPER_DIR/$name" "$SYMLINK_DIR/$name"
}

# Remove wrapper script for flatpak app
remove_flatpak_wrapper() {
    local name="$1"
    rm -f "$FLATPAK_WRAPPER_DIR/$name" "$SYMLINK_DIR/$name"
}

# Extract short name from app ID (org.gimp.GIMP → gimp)
get_flatpak_shortname() {
    local app_id="$1"
    echo "${app_id##*.}" | tr '[:upper:]' '[:lower:]'
}

# Clean up wrappers for uninstalled flatpak apps
cleanup_flatpak_wrappers() {
    [[ ! -d "$FLATPAK_WRAPPER_DIR" ]] && return 0

    # Get list of currently installed flatpak app short names
    local -a current_apps=()
    while read -r app_id; do
        [[ -z "$app_id" ]] && continue
        current_apps+=("$(get_flatpak_shortname "$app_id")")
    done < <(flatpak list --app --columns=application 2>/dev/null)

    # Remove wrappers for apps that no longer exist
    for wrapper in "$FLATPAK_WRAPPER_DIR"/*; do
        [[ ! -f "$wrapper" ]] && continue
        local name=$(basename "$wrapper")

        # Check if this app is still installed
        local found=0
        for app in "${current_apps[@]}"; do
            [[ "$name" == "$app" ]] && found=1 && break
        done

        if [[ $found -eq 0 ]]; then
            remove_flatpak_wrapper "$name"
        fi
    done
}

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

    local app_id=""

    # If it looks like an app ID (has dots), use directly
    if [[ "$tool" == *.*.* ]]; then
        app_id="$tool"
        flatpak install -y flathub "$app_id" || return 1
    else
        # Search using existing function and extract app ID
        local result=$(search_flatpak "$tool" | head -1)
        [[ -z "$result" ]] && return 1

        app_id=$(echo "$result" | awk '{print $1}')
        flatpak install -y flathub "$app_id" || return 1
    fi

    # Create wrapper script for convenient launching
    local shortname=$(get_flatpak_shortname "$app_id")
    create_flatpak_wrapper "$shortname" "$app_id"

    return 0
}

# Uninstall flatpak package
# Handles both app names and full app IDs (org.app.Name format)
uninstall_flatpak() {
    local pkg="$1"
    local source="$2"

    local app_id=""

    # If source has metadata (flatpak:org.app.Name), use that
    if [[ "$source" == flatpak:* ]]; then
        app_id="${source#flatpak:}"
        flatpak uninstall -y "$app_id" || return 1
    else
        # Try to find full app ID from installed apps
        app_id=$(flatpak list --app --columns=application 2>/dev/null | grep -i "$pkg" | head -1)
        if [[ -n "$app_id" ]]; then
            flatpak uninstall -y "$app_id" || return 1
        else
            # Fallback: try package name directly
            flatpak uninstall -y "$pkg" || return 1
        fi
    fi

    # Remove wrapper script
    if [[ -n "$app_id" ]]; then
        local shortname=$(get_flatpak_shortname "$app_id")
        remove_flatpak_wrapper "$shortname"
    fi

    return 0
}

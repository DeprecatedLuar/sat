#!/usr/bin/env bash
# sat clone - clone repositories to current directory

# Ensure user info cache exists
_ensure_user_info() {
    local user_info="$SAT_DATA/user-info"

    # Return if cache exists
    [[ -f "$user_info" ]] && return 0

    # Ensure directory exists
    mkdir -p "$SAT_DATA"

    # Try to get GitHub username from git config
    local github_user
    github_user=$(git config --global github.user 2>/dev/null)

    # If not found, prompt the user
    if [[ -z "$github_user" ]]; then
        echo "Btw what's your github username? (just to enable quick cloning):"
        read github_user
        [[ -z "$github_user" ]] && { echo "Error: GitHub username required" >&2; return 1; }
    fi

    # Write to cache
    echo "GITHUB_USER=$github_user" > "$user_info"
}

sat_clone() {
    local input="$1"

    [[ -z "$input" ]] && { echo "Usage: sat clone <repo>"; return 1; }

    # Ensure user info exists and load it
    _ensure_user_info || return 1
    source "$SAT_DATA/user-info"

    # Determine if input is full path (owner/repo) or short name
    local repo
    if [[ "$input" == */* ]]; then
        # Full path provided
        repo="$input"
    else
        # Short name - use cached username
        repo="$GITHUB_USER/$input"
    fi

    echo "Cloning $repo..."
    git clone "https://github.com/$repo.git" "${input##*/}"
}

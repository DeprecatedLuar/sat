#!/usr/bin/env bash
# sat internal - Manifest API (internal use only)

# ══════════════════════════════════════════════════════════════════════════════
# ──[MANIFEST FUNCTIONS]────────────────────────────────────────────────────────
# ══════════════════════════════════════════════════════════════════════════════
# Internal functions for manifest manipulation. Used by the API.

# sat-manifest (system manifest: tool=source)
_sat_manifest_add()    { sed -i "/^$1=/d" "$SAT_MANIFEST" 2>/dev/null; echo "$1=$2" >> "$SAT_MANIFEST"; }
_sat_manifest_get()    { grep "^$1=" "$SAT_MANIFEST" 2>/dev/null | cut -d= -f2; }
_sat_manifest_remove() { sed -i "/^$1=/d" "$SAT_MANIFEST"; }
_sat_manifest_has()    { grep -q "^$1=" "$SAT_MANIFEST" 2>/dev/null; }

# shell-manifest (master manifest: tool:source:pid)
_shell_manifest_add()        { echo "$1:$2:$3" >> "$SAT_SHELL_MASTER"; }
_shell_manifest_pids()       { grep "^$1:$2:" "$SAT_SHELL_MASTER" 2>/dev/null | cut -d: -f3; }
_shell_manifest_has()        { grep -q "^$1:" "$SAT_SHELL_MASTER" 2>/dev/null; }
_shell_manifest_remove()     { sed -i "\|^$1:$2:$3\$|d" "$SAT_SHELL_MASTER"; }
_shell_manifest_remove_all() { sed -i "\|^$1:|d" "$SAT_SHELL_MASTER"; }
_shell_manifest_promote() {
    local tool="$1" src="$2"
    sed -i "\|^${tool}:${src}:|d" "$SAT_SHELL_MASTER"
    _sat_manifest_has "$tool" || _sat_manifest_add "$tool" "$src"
}

# pid-manifest (session manifest: TOOL=x, SOURCE_x=y)
_pid_manifest_add() {
    local pid="$1" tool="$2" src="$3"
    mkdir -p "$SAT_SHELL_DIR/$pid"
    echo "TOOL=$tool" >> "$SAT_SHELL_DIR/$pid/manifest"
    echo "SOURCE_$tool=$src" >> "$SAT_SHELL_DIR/$pid/manifest"
}
_pid_manifest_tools()  { [[ -f "$SAT_SHELL_DIR/$1/manifest" ]] && grep "^TOOL=" "$SAT_SHELL_DIR/$1/manifest" | cut -d= -f2; }
_pid_manifest_source() { [[ -f "$SAT_SHELL_DIR/$1/manifest" ]] && grep "^SOURCE_$2=" "$SAT_SHELL_DIR/$1/manifest" | cut -d= -f2; }
_pid_manifest_remove() { rm -rf "$SAT_SHELL_DIR/$1" "/tmp/sat-$1" 2>/dev/null; }

# ══════════════════════════════════════════════════════════════════════════════
# ──[INTERNAL API ROUTER]───────────────────────────────────────────────────────
# ══════════════════════════════════════════════════════════════════════════════

sat_internal() {
    case "$1" in
        sat-manifest)
            case "$2" in
                add)
                    [[ -z "$3" || -z "$4" ]] && { echo "Usage: sat internal sat-manifest add <tool> <source>"; exit 1; }
                    _sat_manifest_add "$3" "$4"
                    ;;
                get)
                    [[ -z "$3" ]] && { echo "Usage: sat internal sat-manifest get <tool>"; exit 1; }
                    _sat_manifest_get "$3"
                    ;;
                remove)
                    [[ -z "$3" ]] && { echo "Usage: sat internal sat-manifest remove <tool>"; exit 1; }
                    _sat_manifest_remove "$3"
                    ;;
                has)
                    [[ -z "$3" ]] && { echo "Usage: sat internal sat-manifest has <tool>"; exit 1; }
                    _sat_manifest_has "$3"
                    ;;
                list)
                    [[ -f "$SAT_MANIFEST" ]] && cat "$SAT_MANIFEST"
                    ;;
                *)
                    echo "Usage: sat internal sat-manifest <add|get|remove|has|list> [args]"; exit 1
                    ;;
            esac
            ;;
        shell-manifest)
            case "$2" in
                add)
                    [[ -z "$3" || -z "$4" || -z "$5" ]] && { echo "Usage: sat internal shell-manifest add <tool> <source> <pid>"; exit 1; }
                    _shell_manifest_add "$3" "$4" "$5"
                    ;;
                pids)
                    [[ -z "$3" || -z "$4" ]] && { echo "Usage: sat internal shell-manifest pids <tool> <source>"; exit 1; }
                    _shell_manifest_pids "$3" "$4"
                    ;;
                has)
                    [[ -z "$3" ]] && { echo "Usage: sat internal shell-manifest has <tool>"; exit 1; }
                    _shell_manifest_has "$3"
                    ;;
                remove)
                    [[ -z "$3" || -z "$4" || -z "$5" ]] && { echo "Usage: sat internal shell-manifest remove <tool> <source> <pid>"; exit 1; }
                    _shell_manifest_remove "$3" "$4" "$5"
                    ;;
                remove-all)
                    [[ -z "$3" ]] && { echo "Usage: sat internal shell-manifest remove-all <tool>"; exit 1; }
                    _shell_manifest_remove_all "$3"
                    ;;
                promote)
                    [[ -z "$3" || -z "$4" ]] && { echo "Usage: sat internal shell-manifest promote <tool> <source>"; exit 1; }
                    _shell_manifest_promote "$3" "$4"
                    ;;
                list)
                    [[ -f "$SAT_SHELL_MASTER" ]] && cat "$SAT_SHELL_MASTER"
                    ;;
                *)
                    echo "Usage: sat internal shell-manifest <add|pids|has|remove|remove-all|promote|list> [args]"; exit 1
                    ;;
            esac
            ;;
        pid-manifest)
            case "$2" in
                add)
                    [[ -z "$3" || -z "$4" || -z "$5" ]] && { echo "Usage: sat internal pid-manifest add <pid> <tool> <source>"; exit 1; }
                    _pid_manifest_add "$3" "$4" "$5"
                    ;;
                tools)
                    [[ -z "$3" ]] && { echo "Usage: sat internal pid-manifest tools <pid>"; exit 1; }
                    _pid_manifest_tools "$3"
                    ;;
                source)
                    [[ -z "$3" || -z "$4" ]] && { echo "Usage: sat internal pid-manifest source <pid> <tool>"; exit 1; }
                    _pid_manifest_source "$3" "$4"
                    ;;
                remove)
                    [[ -z "$3" ]] && { echo "Usage: sat internal pid-manifest remove <pid>"; exit 1; }
                    _pid_manifest_remove "$3"
                    ;;
                *)
                    echo "Usage: sat internal pid-manifest <add|tools|source|remove> [args]"; exit 1
                    ;;
            esac
            ;;
        *)
            echo "Usage: sat internal <sat-manifest|shell-manifest|pid-manifest> <operation> [args]"; exit 1
            ;;
    esac
}

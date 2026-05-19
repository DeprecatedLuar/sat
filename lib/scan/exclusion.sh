#!/usr/bin/env bash
# exclusion.sh - Centralized exclusion patterns for package scanning

# User config file location
SAT_EXCLUDE_CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/sat/exclude"

# Default exclusion patterns (infrastructure, not user apps)
# Users can override/extend via ~/.config/sat/exclude
DEFAULT_EXCLUSIONS=(
    # Filesystem utilities (dozens of binaries each)
    'xfs_*' 'btrfs-*' 'fsck.*' 'mkfs.*' 'e2*' 'resize*'

    # Network & WiFi infrastructure
    'wpa_*' 'iw*' 'dhcp*'

    # Bluetooth infrastructure
    'bluez-*' 'bluetooth*'

    # System services & daemons
    'systemd*' 'udev*' 'polkit*'

    # X11/Wayland low-level utilities
    'xdg-*' 'setxkbmap' 'xmodmap' 'xkbcomp'

    # Hardware/driver utilities
    'isdv4-*' 'xsetwacom' 'xgamma' 'xbacklight'

    # Audio low-level utilities
    'alsa-*' 'pulse*' 'pactl' 'pacmd'
)

# Load user exclusion patterns (cached)
_load_exclusions() {
    [[ -n "$_EXCLUSIONS_LOADED" ]] && return
    _EXCLUSIONS_LOADED=1

    # Start with defaults
    EXCLUSION_PATTERNS=("${DEFAULT_EXCLUSIONS[@]}")

    # Extend with user patterns if config exists
    if [[ -f "$SAT_EXCLUDE_CONFIG" ]]; then
        while IFS= read -r pattern; do
            [[ -z "$pattern" || "$pattern" =~ ^[[:space:]]*# ]] && continue
            EXCLUSION_PATTERNS+=("$pattern")
        done < "$SAT_EXCLUDE_CONFIG"
    fi
}

# Check if program should be excluded from scanning
# Args: prog, source
# Returns: 0 if excluded, 1 if should be included
is_excluded() {
    local prog="$1" src="$2"

    # Load patterns on first call
    _load_exclusions

    # Check user + default patterns (glob matching)
    for pattern in "${EXCLUSION_PATTERNS[@]}"; do
        # shellcheck disable=SC2053
        [[ "$prog" == $pattern ]] && return 0
    done

    # Global exclusions (all sources)
    case "$prog" in
        .*|_*|*-config|*-settings) return 0 ;;
    esac

    # Per-source exclusions (hardcoded safety patterns)
    case "$src" in
        cargo)
            case "$prog" in
                cargo-*|clippy-driver|rust[!u]*|rls) return 0 ;;
            esac
            ;;
        nix)
            case "$prog" in
                nix|nix-*) return 0 ;;
            esac
            ;;
        nixos|pacman|apt|dnf|apk)
            # System package manager exclusions (infrastructure, not user apps)

            # Package manager tools themselves
            case "$prog" in
                nix|nix-*|nixos-*) return 0 ;;  # NixOS
                pacman|makepkg|pacman-*) return 0 ;;  # Arch
                apt|apt-*|dpkg|dpkg-*|aptitude) return 0 ;;  # Debian/Ubuntu
                dnf|yum|rpm|rpm-*) return 0 ;;  # Fedora/RHEL
                apk) return 0 ;;  # Alpine
            esac

            # Desktop environment infrastructure (services/daemons, not apps)
            case "$prog" in
                # XFCE services
                xfce4-panel|xfce4-session|xfce4-power-manager|xfce4-settings|xfdesktop|xfwm4|xfce4-screensaver) return 0 ;;
                # GNOME services
                gnome-keyring|gnome-settings-daemon|gnome-session|gvfs*) return 0 ;;
                # KDE/Plasma services
                plasma-*|kwin*|kdeinit*) return 0 ;;
                # KDE infrastructure tools
                bluedevil-sendfile|bluedevil-wizard|obex-client-tool|obex-server-tool|obexctl) return 0 ;;
                servicemenuinstaller|kdialog_progress_helper|kscreen-console|exec_inspect.sh) return 0 ;;
                # Plasma desktop utilities
                kaccess|kapplymousetheme|kcm-touchpad-list-devices|knetattach) return 0 ;;
                krunner-plugininstaller|solid-action-desktop-gen|tastenbrett) return 0 ;;
                # Plasma login/session
                plasmalogin|startplasma-login-wayland) return 0 ;;
            esac

            # X11/Wayland system utilities (not apps)
            case "$prog" in
                xinit|xinput|xlsclients|xprop|xrandr|xrdb|xset|xsetroot|xterm) return 0 ;;
                wayland-scanner|weston-*) return 0 ;;
            esac

            # System daemons and infrastructure
            case "$prog" in
                systemd|systemctl|journalctl|udevadm|dbus-*|polkit*) return 0 ;;
                sudo|su|login|getty|hostname|which) return 0 ;;
                # sudo helpers (keep sudo and sudoedit user tools)
                cvtsudoers|sudo_logsrvd|sudo_sendlog|sudoreplay|visudo) return 0 ;;
            esac

            # Audio infrastructure (ALSA, PulseAudio already covered by patterns)
            case "$prog" in
                aconnect|alsamixer|alsactl|alsaloop|alsatplg|alsaucm|amidi|amixer) return 0 ;;
                aplay|aplaymidi|aplaymidi2|arecord|arecordmidi|arecordmidi2) return 0 ;;
                aseqdump|aseqnet|aseqsend|axfer|iecset|nhlt-dmic-info|speaker-test) return 0 ;;
                # PipeWire infrastructure (keep wpctl user tool)
                pipewire-pulse|wireplumber|wpexec) return 0 ;;
            esac

            # Bluetooth infrastructure (full stack)
            case "$prog" in
                advtest|avinfo|avtest|bcmfw|bdaddr|bluemoon|bluetooth-player|bluetoothctl) return 0 ;;
                bneptest|btattach|btconfig|btgatt-client|btgatt-server|btinfo|btiotest) return 0 ;;
                btmgmt|btmon|btpclient|btpclientctl|btproxy|btsnoop|check-selftest) return 0 ;;
                cltest|create-image|eddystone|gatt-service) return 0 ;;
                hcieventmask|hcisecfilter|hex2hcd|hid2hci|hwdb|ibeacon|isotest) return 0 ;;
                l2ping|l2test|mpris-proxy|nokfw|oobtest|rctest|rtlfw|scotest) return 0 ;;
                seq2bseq|test-runner) return 0 ;;
            esac

            # Network infrastructure (DNS, DHCP, firewall)
            case "$prog" in
                # BIND DNS tools
                arpaname|ddns-confgen|delv|dig|dnssec-*|host|mdig|named|named-*) return 0 ;;
                nsec3hash|nslookup|nsupdate|rndc|rndc-confgen|tsig-keygen) return 0 ;;
                # dnsmasq helpers
                dhcp_lease_time|dhcp_release|dhcp_release6) return 0 ;;
                # iptables/netfilter (match base commands and variants)
                arptables*|ebtables*|ip6tables*|iptables*) return 0 ;;
                nfbpf_compile|nfnl_osf|xtables-*) return 0 ;;
                # iwd
                hwsim|iwctl|iwmon) return 0 ;;
                # inetutils (legacy network tools)
                dnsdomainname|ftp|ftpd|rcp|rlogin|rlogind|rsh|rshd) return 0 ;;
                talk|talkd|telnet|telnetd) return 0 ;;
                # NetworkManager infrastructure (not user tools like nmcli/nmtui)
                NetworkManager|nm-online) return 0 ;;
                # netctl (Arch network manager)
                netctl|netctl-auto|wifi-menu) return 0 ;;
                # NFS infrastructure
                exportfs|fsidd|mount.nfs*|mountstats|nfsconf|nfsd*|nfsidmap) return 0 ;;
                nfsiostat|nfsrahead|nfsref|nfsstat|nfsv4.exportd) return 0 ;;
                rpc.*|rpcctl|rpcdebug|showmount|sm-notify|start-statd|umount.nfs*) return 0 ;;
                # ModemManager infrastructure
                ModemManager|mmcli) return 0 ;;
                # OpenSSH daemon
                sshd|findssl.sh) return 0 ;;
                # VPN infrastructure
                tailscaled|xl2tpd|xl2tpd-control) return 0 ;;
            esac

            # Filesystem utilities (beyond glob patterns)
            case "$prog" in
                # btrfs (already has btrfs-* pattern, add root command)
                btrfs|btrfsck|btrfstune) return 0 ;;
                # Snapper daemon (keep snapper user tool)
                snapperd|mksubvolume|snbk) return 0 ;;
                # dosfs/FAT
                dosfsck|dosfslabel|fatlabel) return 0 ;;
                # exfat
                dump.exfat|exfat2img|exfatlabel|tune.exfat) return 0 ;;
                # f2fs
                defrag.f2fs|dump.f2fs|f2fs_io|f2fscrypt|f2fslabel|fibmap.f2fs) return 0 ;;
                parse.f2fs|sload.f2fs) return 0 ;;
                # jfs
                jfs_*) return 0 ;;
                # nilfs2
                chcp|dumpseg|lscp|lssu|mkcp|nilfs-*|nilfs_cleanerd|rmcp) return 0 ;;
                # Device mapper / LVM (specific patterns to avoid catching 'pv' pipe viewer)
                blkdeactivate|dmeventd|dmevent_tool|dmsetup|dmstats|dmvdostats|dmraid) return 0 ;;
                fsadm|lv[a-z]*|pv[a-z]*|vg[a-z]*) return 0 ;;
                # RAID (mdadm)
                mdadm|mdmon|raid6check) return 0 ;;
                # Cryptsetup
                cryptsetup|integritysetup|veritysetup) return 0 ;;
                # FUSE
                fusermount|mount.fuse|ulockmgr_server) return 0 ;;
            esac

            # Hardware/firmware tools
            case "$prog" in
                # dmidecode suite
                biosdecode|dmidecode|ownership|vpddecode) return 0 ;;
                # Disk tools
                ethtool|hdparm|idectl|ultrabayd|wiper.sh|lsscsi|systool) return 0 ;;
                # Hardware detection
                hwdetect|check_hd|convert_hd|getsysinfo|mk_isdnhwdb) return 0 ;;
                # Firmware updates (infrastructure, not the fwupd/fwupdmgr user tools)
                fwupd-dbxtool|fwupdtool) return 0 ;;
                # SCSI utilities (sg3_utils - 100+ tools)
                scsi_*|sg_*|sginfo|sgm_dd|sgp_dd) return 0 ;;
                # SMART monitoring
                smartd|update-smart-drivedb) return 0 ;;
                # USB utilities
                lsusb|lsusb.py|usb-devices|usbhid-dump|usbreset) return 0 ;;
                usb_modeswitch|usb_modeswitch_dispatcher) return 0 ;;
                # Wacom tablet driver
                isdv4-*) return 0 ;;
                # Power management
                upower|powerprofilesctl) return 0 ;;
            esac

            # Boot/EFI infrastructure
            case "$prog" in
                # EFI utilities
                efibootdump|cert-to-efi-*|efi-readvar|efi-updatevar|efitool-mkusb) return 0 ;;
                flash-var|hash-to-efi-sig-list|sig-list-to-certs|sign-efi-sig-list) return 0 ;;
                # Bootloader tools
                limine-*|mkinitcpio|update-initramfs) return 0 ;;
                # Plymouth boot splash
                plymouth|plymouthd|plymouth-set-default-theme|kplymouththemeinstaller) return 0 ;;
            esac

            # Docker infrastructure (not docker/docker-compose user tools)
            case "$prog" in
                docker-init|docker-proxy|dockerd) return 0 ;;
            esac

            # Flatpak debugging tools (not flatpak user tool)
            case "$prog" in
                flatpak-bisect|flatpak-coredumpctl) return 0 ;;
            esac

            # Gaming infrastructure daemons (not gamemode user tools)
            case "$prog" in
                gamemoded) return 0 ;;
            esac

            # CachyOS-specific infrastructure
            case "$prog" in
                arch-update|cachy-update|update-initramfs) return 0 ;;
                cachyos-bugreport.sh|cachyos-pi|sbctl-batch-sign) return 0 ;;
                dlss-swapper|dlss-swapper-dll|game-performance|kerver) return 0 ;;
                paste-cachyos|pci-latency|topmem|zink-run) return 0 ;;
            esac

            # System utilities (not user-facing tools)
            case "$prog" in
                # Diffutils
                cmp|diff|diff3|sdiff) return 0 ;;
                # Manual page infrastructure
                accessdb|apropos|catman|lexgrog|man-recode|mandb|manpath|whatis) return 0 ;;
                # Texinfo documentation tools
                install-info|makeinfo|pdftexi2dvi|pod2texi|texi2any|texi2dvi|texi2pdf|texindex) return 0 ;;
                # Misc infrastructure
                logrotate|lsb_release|lsinitcpio|xsettingsd|dump_xsettings) return 0 ;;
                # Thumbnail generators
                ffmpegthumbnailer|gsf|gsf-office-thumbnailer|gsf-vba-dump) return 0 ;;
                mypaint-ora-thumbnailer) return 0 ;;
                # Pager helpers (keep 'less' itself, exclude helpers)
                lessecho|lesskey) return 0 ;;
                # Legacy floppy/DOS tools (mtools)
                amuFormat.sh|floppyd|floppyd_installtest|lz|mattrib|mbadblocks) return 0 ;;
                mcat|mcd|mcheck|mcomp|mcopy|mdel|mdeltree|mdir|mdoctorfat) return 0 ;;
                mdu|mformat|minfo|mkmanifest|mlabel|mmd|mmount|mmove) return 0 ;;
                mpartition|mrd|mren|mshortname|mshowfat|mtoolstest) return 0 ;;
                mtype|mxtar|mzip|tgz|uz) return 0 ;;
                # Mesa test/debug utilities
                eglgears_*|eglinfo|eglkms|egltri_*|es2_info|es2gears_*|es2tri) return 0 ;;
                glxgears|glxinfo|peglgears|vkgears|xeglgears|xeglthreads) return 0 ;;
                # OpenColorIO CLI tools
                ocio*) return 0 ;;
                # OBS helper tools (keep 'obs' main binary)
                obs-ffmpeg-mux|obs-nvenc-test) return 0 ;;
                # Boot detection
                linux-boot-prober|os-prober) return 0 ;;
                # Pacman helper utilities (including pkgfile)
                checkupdates|checkrebuild|paccache|pacdiff|paclist|paclog-pkglist) return 0 ;;
                pacscripts|pacsearch|pacsort|pactree|rankmirrors|updpkgsums) return 0 ;;
                pkgfile|pkgfiled) return 0 ;;
                # File search databases
                locate|mlocate|plocate|plocate-build|updatedb) return 0 ;;
                # Python/Ruby infrastructure helpers
                idle[0-9]*|pydoc[0-9]*|python[0-9]*|websockets) return 0 ;;
                syntax_suggest) return 0 ;;
                # Archive helper tools (keep main unzip/funzip, exclude specialized ones)
                unzipsfx|zipgrep|zipinfo) return 0 ;;
            esac
            ;;
        *)
            # Git infrastructure
            [[ "$prog" == git-* || "$prog" == scalar || "$prog" == trash-* ]] && return 0
            # Language servers (infrastructure)
            case "$prog" in
                gopls|rust-analyzer|typescript-language-server|pyright) return 0 ;;
            esac
            # sat internal dependencies
            [[ "$prog" == huber ]] && return 0
            ;;
    esac

    return 1
}

# Exclusion patterns array (populated on first call to is_excluded)
declare -a EXCLUSION_PATTERNS=()

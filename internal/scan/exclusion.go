package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Environment variables
	EnvXDGConfigHome = "XDG_CONFIG_HOME"
	EnvHome          = "HOME"

	// Paths
	DefaultConfigDir = ".config"
	SatConfigDir     = "sat"
	ExcludeFileName  = "exclude"

	// Comment and prefix patterns
	CommentPrefix   = "#"
	DotPrefix       = '.'
	UnderscorePrefix = '_'

	// Common suffixes to exclude
	ConfigSuffix   = "-config"
	SettingsSuffix = "-settings"

	// Cargo-specific exclusions
	CargoPrefix     = "cargo-"
	ClippyDriver    = "clippy-driver"
	RustPrefix      = "rust"
	RustuPrefix     = "rustu"
	RLS             = "rls"

	// Nix-specific exclusions
	NixTool    = "nix"
	NixPrefix  = "nix-"

	// Git infrastructure
	GitPrefix   = "git-"
	ScalarTool  = "scalar"
	TrashPrefix = "trash-"

	// Language servers
	Gopls                     = "gopls"
	RustAnalyzer              = "rust-analyzer"
	TypeScriptLanguageServer  = "typescript-language-server"
	Pyright                   = "pyright"

	// Sat internal dependencies
	Huber = "huber"

	// Source types
	SourceCargo  = "cargo"
	SourceNix    = "nix"
	SourceNixOS  = "nixos"
	SourcePacman = "pacman"
	SourceAPT    = "apt"
	SourceDNF    = "dnf"
	SourceAPK    = "apk"
	SourceSystem = "system"
)

var (
	exclusionsLoaded  bool
	exclusionPatterns []string
)

// Default exclusion patterns (infrastructure, not user apps)
var defaultExclusions = []string{
	// Filesystem utilities
	"xfs_*", "btrfs-*", "fsck.*", "mkfs.*", "e2*", "resize*",

	// Network & WiFi infrastructure
	"wpa_*", "iw*", "dhcp*",

	// Bluetooth infrastructure
	"bluez-*", "bluetooth*",

	// System services & daemons
	"systemd*", "udev*", "polkit*",

	// X11/Wayland low-level utilities
	"xdg-*", "setxkbmap", "xmodmap", "xkbcomp",

	// Hardware/driver utilities
	"isdv4-*", "xsetwacom", "xgamma", "xbacklight",

	// Audio low-level utilities
	"alsa-*", "pulse*", "pactl", "pacmd", "alsabat*",

	// Network infrastructure
	"nm*",

	// CPU/Power management
	"cpufreq-*",
}

// LoadExclusions loads user and default exclusion patterns
func LoadExclusions() {
	if exclusionsLoaded {
		return
	}
	exclusionsLoaded = true

	// Start with defaults
	exclusionPatterns = make([]string, len(defaultExclusions))
	copy(exclusionPatterns, defaultExclusions)

	// Load user patterns from config
	configHome := os.Getenv(EnvXDGConfigHome)
	if configHome == "" {
		configHome = filepath.Join(os.Getenv(EnvHome), DefaultConfigDir)
	}
	userExcludeFile := filepath.Join(configHome, SatConfigDir, ExcludeFileName)

	file, err := os.Open(userExcludeFile)
	if err != nil {
		return // Config file doesn't exist, that's OK
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, CommentPrefix) {
			continue
		}
		exclusionPatterns = append(exclusionPatterns, line)
	}
}

// IsExcluded checks if a program should be excluded from scanning
func IsExcluded(prog, source string) bool {
	// Load patterns on first call
	LoadExclusions()

	// Check user + default patterns (glob matching)
	for _, pattern := range exclusionPatterns {
		if matchGlob(prog, pattern) {
			return true
		}
	}

	// Global exclusions (all sources)
	if len(prog) > 0 && (prog[0] == DotPrefix || prog[0] == UnderscorePrefix) {
		return true
	}
	if strings.HasSuffix(prog, ConfigSuffix) || strings.HasSuffix(prog, SettingsSuffix) {
		return true
	}

	// Per-source exclusions (hardcoded safety patterns)
	switch source {
	case SourceCargo:
		if strings.HasPrefix(prog, CargoPrefix) || prog == ClippyDriver ||
			(strings.HasPrefix(prog, RustPrefix) && !strings.HasPrefix(prog, RustuPrefix)) ||
			prog == RLS {
			return true
		}

	case SourceNix:
		if prog == NixTool || strings.HasPrefix(prog, NixPrefix) {
			return true
		}

	case SourceNixOS, SourcePacman, SourceAPT, SourceDNF, SourceAPK, SourceSystem:
		return isSystemInfrastructure(prog)

	default:
		// Git infrastructure
		if strings.HasPrefix(prog, GitPrefix) || prog == ScalarTool || strings.HasPrefix(prog, TrashPrefix) {
			return true
		}
		// Language servers
		switch prog {
		case Gopls, RustAnalyzer, TypeScriptLanguageServer, Pyright:
			return true
		}
		// sat internal dependencies
		if prog == Huber {
			return true
		}
	}

	return false
}

// isSystemInfrastructure checks if a program is system infrastructure
func isSystemInfrastructure(prog string) bool {
	// Package manager tools
	switch prog {
	case "nix", "nixos-rebuild", "pacman", "makepkg", "apt", "apt-get", "apt-cache",
		"dpkg", "aptitude", "dnf", "yum", "rpm", "apk":
		return true
	}
	if strings.HasPrefix(prog, "nix-") || strings.HasPrefix(prog, "nixos-") ||
		strings.HasPrefix(prog, "pacman-") || strings.HasPrefix(prog, "apt-") ||
		strings.HasPrefix(prog, "dpkg-") || strings.HasPrefix(prog, "rpm-") {
		return true
	}

	// Desktop environment services (not user apps)
	switch prog {
	case "xfce4-panel", "xfce4-session", "xfce4-power-manager", "xfce4-settings",
		"xfdesktop", "xfwm4", "xfce4-screensaver",
		"gnome-keyring", "gnome-settings-daemon", "gnome-session":
		return true
	}
	if strings.HasPrefix(prog, "gvfs") || strings.HasPrefix(prog, "plasma-") ||
		strings.HasPrefix(prog, "kwin") || strings.HasPrefix(prog, "kdeinit") {
		return true
	}

	// KDE infrastructure
	switch prog {
	case "bluedevil-sendfile", "bluedevil-wizard", "obex-client-tool", "obex-server-tool",
		"obexctl", "servicemenuinstaller", "kdialog_progress_helper", "kscreen-console",
		"exec_inspect.sh", "kaccess", "kapplymousetheme", "kcm-touchpad-list-devices",
		"knetattach", "krunner-plugininstaller", "solid-action-desktop-gen", "tastenbrett",
		"plasmalogin", "startplasma-login-wayland":
		return true
	}

	// X11/Wayland system utilities
	switch prog {
	case "xinit", "xinput", "xlsclients", "xprop", "xrandr", "xrdb", "xset", "xsetroot",
		"xterm", "wayland-scanner":
		return true
	}
	if strings.HasPrefix(prog, "weston-") {
		return true
	}

	// System daemons
	switch prog {
	case "systemd", "systemctl", "journalctl", "udevadm", "sudo", "su", "login",
		"getty", "hostname", "which", "cvtsudoers", "sudo_logsrvd", "sudo_sendlog", "sudoreplay":
		return true
	}
	if strings.HasPrefix(prog, "dbus-") || strings.HasPrefix(prog, "polkit") {
		return true
	}

	// Audio infrastructure
	switch prog {
	case "aconnect", "alsamixer", "alsactl", "alsaloop", "alsatplg", "alsaucm", "amidi", "amixer",
		"aplay", "aplaymidi", "aplaymidi2", "arecord", "arecordmidi", "arecordmidi2",
		"aseqdump", "aseqnet", "aseqsend", "axfer", "iecset", "nhlt-dmic-info", "speaker-test",
		"pipewire-pulse", "wireplumber", "wpexec":
		return true
	}

	// Bluetooth infrastructure (comprehensive list)
	switch prog {
	case "advtest", "avinfo", "avtest", "bcmfw", "bdaddr", "bluemoon", "bluetooth-player",
		"bluetoothctl", "bneptest", "btattach", "btconfig", "btgatt-client", "btgatt-server",
		"btinfo", "btiotest", "btmgmt", "btmon", "btpclient", "btpclientctl", "btproxy",
		"btsnoop", "check-selftest", "cltest", "create-image", "eddystone", "gatt-service",
		"hcieventmask", "hcisecfilter", "hex2hcd", "hid2hci", "hwdb", "ibeacon", "isotest",
		"l2ping", "l2test", "mpris-proxy", "nokfw", "oobtest", "rctest", "rtlfw", "scotest",
		"seq2bseq", "test-runner":
		return true
	}

	// Network infrastructure (DNS, DHCP, firewall)
	networkTools := []string{
		"arpaname", "ddns-confgen", "delv", "dig", "host", "mdig", "named", "nsec3hash",
		"nslookup", "nsupdate", "rndc", "rndc-confgen", "tsig-keygen",
		"dhcp_lease_time", "dhcp_release", "dhcp_release6",
		"nfbpf_compile", "nfnl_osf",
		"hwsim", "iwctl", "iwmon",
		"dnsdomainname", "ftp", "ftpd", "rcp", "rlogin", "rlogind", "rsh", "rshd",
		"talk", "talkd", "telnet", "telnetd",
		"NetworkManager", "nm-online", "dnsmasq", "netctl", "netctl-auto", "wifi-menu",
		"ModemManager", "mmcli", "sshd", "findssl.sh", "tailscaled", "xl2tpd", "xl2tpd-control",
	}
	for _, tool := range networkTools {
		if prog == tool {
			return true
		}
	}
	if strings.HasPrefix(prog, "dnssec-") || strings.HasPrefix(prog, "named-") ||
		strings.HasPrefix(prog, "arptables") || strings.HasPrefix(prog, "ebtables") ||
		strings.HasPrefix(prog, "ip6tables") || strings.HasPrefix(prog, "iptables") ||
		strings.HasPrefix(prog, "xtables-") {
		return true
	}

	// NFS infrastructure
	if strings.HasPrefix(prog, "mount.nfs") || strings.HasPrefix(prog, "umount.nfs") ||
		strings.HasPrefix(prog, "nfs") || strings.HasPrefix(prog, "rpc.") {
		return true
	}

	// Filesystem utilities
	fsTools := []string{
		"badblocks", "chattr", "compile_et", "debugfs", "dumpe2fs", "filefrag",
		"logsave", "lsattr", "mk_cmds", "mklost+found", "tune2fs",
		"btrfs", "btrfsck", "btrfstune", "snapperd", "mksubvolume", "snbk",
		"dosfsck", "dosfslabel", "fatlabel", "mkdosfs",
		"dump.exfat", "exfat2img", "exfatlabel", "tune.exfat",
		"defrag.f2fs", "dump.f2fs", "f2fs_io", "f2fscrypt", "f2fslabel", "fibmap.f2fs",
		"parse.f2fs", "sload.f2fs",
		"chcp", "dumpseg", "lscp", "lssu", "mkcp", "rmcp",
		"blkdeactivate", "dmeventd", "dmevent_tool", "dmsetup", "dmstats", "dmvdostats", "dmraid",
		"fsadm", "mdadm", "mdmon", "raid6check",
		"cryptsetup", "integritysetup", "veritysetup",
		"fusermount", "mount.fuse", "ulockmgr_server",
	}
	for _, tool := range fsTools {
		if prog == tool {
			return true
		}
	}
	if strings.HasPrefix(prog, "jfs_") || strings.HasPrefix(prog, "nilfs-") ||
		strings.HasPrefix(prog, "lv") || strings.HasPrefix(prog, "pv") ||
		strings.HasPrefix(prog, "vg") {
		// Avoid catching 'pv' pipe viewer
		if len(prog) > 2 && prog != "pv" {
			return true
		}
	}

	// Hardware/firmware tools
	hwTools := []string{
		"biosdecode", "dmidecode", "ownership", "vpddecode",
		"ethtool", "hdparm", "idectl", "ultrabayd", "wiper.sh", "lsscsi", "systool",
		"hwdetect", "check_hd", "convert_hd", "getsysinfo", "mk_isdnhwdb",
		"hwinfo", "smartctl", "smartd", "cpupower", "update-smart-drivedb",
		"fwupd-dbxtool", "fwupdtool",
		"sginfo", "sgm_dd", "sgp_dd", "rescan-scsi-bus.sh",
		"lsusb", "lsusb.py", "usb-devices", "usbhid-dump", "usbreset",
		"usb_modeswitch", "usb_modeswitch_dispatcher",
		"upower", "powerprofilesctl",
	}
	for _, tool := range hwTools {
		if prog == tool {
			return true
		}
	}
	if strings.HasPrefix(prog, "scsi_") || strings.HasPrefix(prog, "sg_") ||
		strings.HasPrefix(prog, "isdv4-") {
		return true
	}

	// Boot/EFI infrastructure
	bootTools := []string{
		"efibootdump", "efibootmgr", "efitool-mkusb", "flash-var",
		"mkinitcpio", "update-initramfs",
		"plymouth", "plymouthd", "plymouth-set-default-theme", "kplymouththemeinstaller",
	}
	for _, tool := range bootTools {
		if prog == tool {
			return true
		}
	}
	if strings.HasPrefix(prog, "cert-to-efi-") || strings.HasPrefix(prog, "efi-") ||
		strings.HasPrefix(prog, "limine-") {
		return true
	}

	// Container/virtualization infrastructure
	switch prog {
	case "docker-init", "docker-proxy", "dockerd",
		"flatpak-bisect", "flatpak-coredumpctl",
		"gamemoded":
		return true
	}

	// CachyOS-specific infrastructure
	cachyTools := []string{
		"arch-update", "cachy-update", "cachyos-bugreport.sh", "cachyos-pi",
		"sbctl-batch-sign", "dlss-swapper", "dlss-swapper-dll", "game-performance",
		"kerver", "paste-cachyos", "pci-latency", "topmem", "zink-run",
	}
	for _, tool := range cachyTools {
		if prog == tool {
			return true
		}
	}

	// System utilities
	sysUtils := []string{
		"cmp", "diff", "diff3", "sdiff",
		"accessdb", "apropos", "catman", "lexgrog", "man-recode", "mandb", "manpath", "whatis",
		"info", "install-info", "makeinfo", "pdftexi2dvi", "pod2texi", "texi2any",
		"texi2dvi", "texi2pdf", "texindex",
		"logrotate", "lsb_release", "lsinitcpio", "xsettingsd", "dump_xsettings",
		"ffmpegthumbnailer", "gsf", "gsf-office-thumbnailer", "gsf-vba-dump",
		"mypaint-ora-thumbnailer",
		"lessecho", "lesskey",
		"linux-boot-prober", "os-prober",
		"pkgfiled", "plocate-build", "updatedb",
		"websockets", "syntax_suggest",
		"unzipsfx", "zipgrep", "zipinfo",
	}
	for _, tool := range sysUtils {
		if prog == tool {
			return true
		}
	}

	// Pacman helpers (keep: checkupdates, paccache, pacdiff, pactree, checkrebuild)
	if prog == "paclist" || prog == "paclog-pkglist" || prog == "pacscripts" ||
		prog == "pacsearch" || prog == "pacsort" || prog == "rankmirrors" || prog == "updpkgsums" {
		return true
	}

	// Python/Ruby infrastructure
	if strings.HasPrefix(prog, "idle") || strings.HasPrefix(prog, "pydoc") ||
		strings.HasPrefix(prog, "python") && len(prog) > 6 {
		return true
	}

	// Mtools (floppy/DOS tools)
	mtoolsList := []string{
		"amuFormat.sh", "floppyd", "floppyd_installtest", "lz", "mattrib", "mbadblocks",
		"mcat", "mcd", "mcheck", "mcomp", "mcopy", "mdel", "mdeltree", "mdir", "mdoctorfat",
		"mdu", "mformat", "minfo", "mkmanifest", "mlabel", "mmd", "mmount", "mmove",
		"mpartition", "mrd", "mren", "mshortname", "mshowfat", "mtoolstest",
		"mtype", "mxtar", "mzip", "tgz", "uz",
	}
	for _, tool := range mtoolsList {
		if prog == tool {
			return true
		}
	}

	// Mesa/OpenGL test utilities
	mesaTools := []string{
		"eglinfo", "eglkms", "es2_info", "es2tri",
		"glxgears", "glxinfo", "peglgears", "vkgears", "xeglgears", "xeglthreads",
	}
	for _, tool := range mesaTools {
		if prog == tool {
			return true
		}
	}
	if strings.HasPrefix(prog, "eglgears_") || strings.HasPrefix(prog, "egltri_") ||
		strings.HasPrefix(prog, "es2gears_") || strings.HasPrefix(prog, "ocio") {
		return true
	}

	// OBS helper tools (keep main 'obs' binary)
	if prog == "obs-ffmpeg-mux" || prog == "obs-nvenc-test" {
		return true
	}

	return false
}

// matchGlob implements simple glob pattern matching (*, ?)
func matchGlob(str, pattern string) bool {
	return matchGlobImpl(str, pattern, 0, 0)
}

func matchGlobImpl(str, pattern string, sIdx, pIdx int) bool {
	sLen := len(str)
	pLen := len(pattern)

	// End of both string and pattern
	if sIdx == sLen && pIdx == pLen {
		return true
	}

	// Pattern exhausted but string remains
	if pIdx == pLen {
		return false
	}

	// Handle * wildcard
	if pattern[pIdx] == '*' {
		// Try matching zero characters
		if matchGlobImpl(str, pattern, sIdx, pIdx+1) {
			return true
		}
		// Try matching one or more characters
		if sIdx < sLen && matchGlobImpl(str, pattern, sIdx+1, pIdx) {
			return true
		}
		return false
	}

	// String exhausted but pattern has non-* characters
	if sIdx == sLen {
		return false
	}

	// Match single character with ?
	if pattern[pIdx] == '?' {
		return matchGlobImpl(str, pattern, sIdx+1, pIdx+1)
	}

	// Match exact character
	if str[sIdx] == pattern[pIdx] {
		return matchGlobImpl(str, pattern, sIdx+1, pIdx+1)
	}

	return false
}

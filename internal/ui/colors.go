package ui

// ANSI color codes
const (
	Reset         = "\033[0m"
	Dim           = "\033[2m"
	Strikethrough = "\033[9m"

	// Source colors (headers)
	Rust     = "\033[0;91m"                // Bright red - Rust/cargo
	Node     = "\033[0;92m"                // Bright green - Node/npm
	Python   = "\033[0;94m"                // Blue - Python/uv
	System   = "\033[0;97m"                // Bright white - System packages
	Flatpak  = "\033[0;95m"                // Bright magenta - Flatpak
	Repo     = "\033[38;2;180;180;180m"    // Soft gray - GitHub repos
	Sat      = "\033[0;36m"                // Cyan - Sat scripts
	Go       = "\033[0;96m"                // Bright Cyan - Go
	Brew     = "\033[0;93m"                // Bright yellow - Homebrew
	Nix      = "\033[38;2;82;119;195m"     // Dark blue #5277C3 - Nix
	AppImage = "\033[38;2;138;98;185m"     // Purple #8A62B9 - AppImage
	Manual   = "\033[38;2;180;140;100m"    // Warm brown - Manual installs

	// Pastel colors (for package names - lighter tints)
	RustLight     = "\033[38;2;220;160;160m"   // Soft pink-red
	NodeLight     = "\033[38;2;160;210;160m"   // Soft mint
	PythonLight   = "\033[38;2;220;210;160m"   // Soft cream
	SystemLight   = "\033[38;2;160;180;220m"   // Soft sky blue
	FlatpakLight  = "\033[38;2;220;160;220m"   // Soft magenta
	RepoLight     = "\033[38;2;140;140;140m"   // Medium gray
	GoLight       = "\033[38;2;160;210;210m"   // Soft teal
	BrewLight     = "\033[38;2;230;175;130m"   // Soft amber
	NixLight      = "\033[38;2;126;186;228m"   // Light blue #7EBAE4
	AppImageLight = "\033[38;2;188;168;215m"   // Soft lavender
	ManualLight   = "\033[38;2;210;180;150m"   // Soft tan
)

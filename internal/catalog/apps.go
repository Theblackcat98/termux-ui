package catalog

// CommonApp is one curated Termux package offered on the Common Apps screen.
// Unlike core tools these install directly with pkg — no core CLI needed —
// and installed status comes from dpkg -s on Pkg.
type CommonApp struct {
	Name    string // display name, e.g. "Neovim"
	Pkg     string // apt package name, e.g. "neovim"
	Command string // binary it provides when different from Pkg (informational)
	Summary string // one-line description shown in the list
}

// AppCategory groups CommonApps under one section header on the screen.
type AppCategory struct {
	Key  string
	Name string
	Apps []CommonApp
}

// AppCategories is the curated roster for screen 8. Every Pkg is verified to
// exist in the termux-main apt repository; order defines display order.
var AppCategories = []AppCategory{
	{
		Key: "essentials", Name: "Essentials",
		Apps: []CommonApp{
			{"Git", "git", "", "version control"},
			{"OpenSSH", "openssh", "ssh", "SSH client and server (port 8022)"},
			{"Nano", "nano", "", "simple terminal editor"},
			{"Zip", "zip", "", "create .zip archives"},
			{"Unzip", "unzip", "", "extract .zip archives"},
			{"Tree", "tree", "", "directory listing as a tree"},
			{"jq", "jq", "", "JSON processor for the command line"},
			{"Wget", "wget", "", "HTTP/FTP downloader"},
		},
	},
	{
		Key: "editors", Name: "Editors & Viewing",
		Apps: []CommonApp{
			{"Neovim", "neovim", "nvim", "modern Vim-based editor"},
			{"Micro", "micro", "", "intuitive terminal editor"},
			{"Vim", "vim", "", "classic Vi IMproved"},
			{"Less", "less", "", "terminal pager"},
			{"Bat", "bat", "", "cat clone with syntax highlighting"},
			{"Git Delta", "git-delta", "delta", "side-by-side diffs for git"},
			{"Ripgrep", "ripgrep", "rg", "fast recursive search"},
			{"fd", "fd", "fd", "simple, fast alternative to find"},
		},
	},
	{
		Key: "monitor", Name: "Monitoring & Disk",
		Apps: []CommonApp{
			{"Htop", "htop", "", "interactive process viewer"},
			{"Tmux", "tmux", "", "terminal multiplexer"},
			{"Duf", "duf", "", "disk usage in a nice table"},
			{"Ncdu", "ncdu", "", "ncurses disk usage analyzer"},
			{"Progress", "progress", "", "progress bar for coreutils"},
			{"Procs", "procs", "", "modern replacement for ps"},
		},
	},
	{
		Key: "media", Name: "Media & Files",
		Apps: []CommonApp{
			{"FFmpeg", "ffmpeg", "", "record, convert and stream audio/video"},
			{"ImageMagick", "imagemagick", "magick", "image manipulation toolkit"},
			{"Aria2", "aria2", "", "multi-protocol download utility"},
			{"Ranger", "ranger", "", "console file manager with vi keys"},
			{"Rsync", "rsync", "", "fast incremental file transfer"},
		},
	},
	{
		Key: "fun", Name: "Fun & Flair",
		Apps: []CommonApp{
			{"Cmatrix", "cmatrix", "", "the matrix screensaver"},
			{"Cava", "cava", "", "cross-platform audio visualizer"},
			{"Figlet", "figlet", "", "big ASCII text banners"},
			{"Sl", "sl", "", "the steam locomotive"},
			{"Fortune", "fortune", "", "random aphorisms"},
			{"Cowsay", "cowsay", "", "a cow says things"},
		},
	},
}

// AllCommonApps flattens AppCategories in display order.
func AllCommonApps() []CommonApp {
	var all []CommonApp
	for _, c := range AppCategories {
		all = append(all, c.Apps...)
	}
	return all
}

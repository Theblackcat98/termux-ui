// Package catalog holds static knowledge about core-termux modules and tools,
// Termux companion apps, extra-keys presets and boot script templates.
//
// The catalog is a fallback only: live status always comes from parsing
// `core list <module>` (see internal/core). When core reports tools unknown to
// this catalog they are added dynamically by the TUI.
package catalog

// Tool is a single installable unit inside a core module, selected by flag.
type Tool struct {
	Name    string // display name, e.g. "Node.js LTS"
	Flag    string // install flag, e.g. "--nodejs-lts"
	Command string // binary provided, e.g. "node" (informational; empty if n/a)
	GLibc   bool   // requires glibc toolchain; shown with warning badge
}

// Module is one core install target.
type Module struct {
	Key         string // "lang"
	Title       string // "Language Packages"
	Description string
	Tools       []Tool
	ModuleLevel bool // installed as a whole; no per-tool pane (editor, shell, ui, auto)
}

// Modules mirrors core 4.x. Order defines screen order.
var Modules = []Module{
	{
		Key: "lang", Title: "Language Packages",
		Description: "Compilers and language runtimes",
		Tools: []Tool{
			{"Node.js LTS", "--nodejs-lts", "node", false},
			{"Python", "--python", "python", false},
			{"Perl", "--perl", "perl", false},
			{"PHP", "--php", "php", false},
			{"Rust", "--rust", "rustc", false},
			{"C/C++ clang", "--clang", "clang", false},
			{"Go", "--golang", "go", false},
			{"Bun", "--bun", "bun", false},
		},
	},
	{
		Key: "db", Title: "Databases",
		Description: "Database engines and clients",
		Tools: []Tool{
			{"PostgreSQL", "--postgresql", "psql", false},
			{"MariaDB", "--mariadb", "mysql", false},
			{"SQLite", "--sqlite", "sqlite3", false},
			{"MongoDB", "--mongodb", "mongod", false},
			{"Redis", "--redis", "redis-server", false},
		},
	},
	{
		Key: "ai", Title: "AI Tools",
		Description: "CLI coding agents and local inference",
		Tools: []Tool{
			{"OpenCode", "--opencode", "opencode", false},
			{"Claude Code", "--claude-code", "claude", false},
			{"Codex CLI", "--codex", "codex", false},
			{"Gemini CLI", "--gemini-cli", "gemini", false},
			{"Qwen Code", "--qwen-code", "qwen", false},
			{"Ollama", "--ollama", "ollama", false},
		},
	},
	{
		Key: "editor", Title: "Editor",
		Description: "Neovim + NvChad bundle (installed as a whole)",
		ModuleLevel: true,
	},
	{
		Key: "dev", Title: "Development Tools",
		Description: "Everyday CLI utilities",
		Tools: []Tool{
			{"GitHub CLI", "--gh", "gh", false},
			{"Wget", "--wget", "wget", false},
			{"Curl", "--curl", "curl", false},
			{"LSD", "--lsd", "lsd", false},
			{"Bat", "--bat", "bat", false},
			{"Proot", "--proot", "proot", false},
			{"Ncurses Utils", "--ncurses", "", false},
			{"Tmate", "--tmate", "tmate", false},
			{"Tmux", "--tmux", "tmux", false},
			{"OpenSSH", "--openssh", "ssh", false},
			{"Cloudflared", "--cloudflared", "cloudflared", false},
			{"Translate Shell", "--translate", "trans", false},
			{"html2text", "--html2text", "html2text", false},
			{"jq", "--jq", "jq", false},
			{"bc", "--bc", "bc", false},
			{"tree", "--tree", "tree", false},
			{"fzf", "--fzf", "fzf", false},
			{"ImageMagick", "--imagemagick", "magick", false},
			{"shfmt", "--shfmt", "shfmt", false},
			{"make", "--make", "make", false},
			{"udocker", "--udocker", "udocker", true},
			{"Superfile", "--spf", "spf", false},
		},
	},
	{
		Key: "npm", Title: "Node.js Global Modules",
		Description: "Global npm packages (some need the glibc toolchain)",
		Tools: []Tool{
			{"TypeScript", "--typescript", "tsc", false},
			{"NestJS CLI", "--nestjs", "nest", false},
			{"Prettier", "--prettier", "prettier", false},
			{"Live Server", "--live-server", "live-server", false},
			{"Localtunnel", "--localtunnel", "lt", false},
			{"Vercel CLI", "--vercel", "vercel", false},
			{"Markserv", "--markserv", "markserv", false},
			{"PSQL Format", "--psqlformat", "psqlformat", false},
			{"NPM Check Updates", "--ncu", "ncu", false},
			{"Ngrok", "--ngrok", "ngrok", false},
			{"Turbopack", "--turbopack", "next-turbopack", true},
		},
	},
	{
		Key: "shell", Title: "ZSH Shell Plugins",
		Description: "ZSH + powerlevel10k + plugin roster (installed as a whole)",
		ModuleLevel: true,
	},
	{
		Key: "ui", Title: "UI Components",
		Description: "Termux UI components (installed as a whole)",
		ModuleLevel: true,
	},
	{
		Key: "auto", Title: "Automation",
		Description: "n8n workflow automation (installed as a whole)",
		ModuleLevel: true,
	},
}

// ModuleByKey returns the module with the given key, or nil.
func ModuleByKey(key string) *Module {
	for i := range Modules {
		if Modules[i].Key == key {
			return &Modules[i]
		}
	}
	return nil
}

// CoreInstallerOneLiner is core's official bootstrap command.
const CoreInstallerOneLiner = "curl -fsSL https://raw.githubusercontent.com/DevCoreXOfficial/core-termux/main/install.sh | bash"

// CompanionApp describes one Termux companion app.
type CompanionApp struct {
	Name       string // "Termux:API"
	CliPackage string // pkg name providing the CLI part ("termux-api")
	FDroidURL  string
	Optional   bool   // Termux:Styling duplicates features of this TUI
	SelfTest   string // human description of what self-test does
}

// CompanionApps are listed on Screen 7. All must come from F-Droid.
var CompanionApps = []CompanionApp{
	{Name: "Termux:API", CliPackage: "termux-api",
		FDroidURL: "https://f-droid.org/en/packages/com.termux.api/",
		SelfTest:  "Runs termux-battery-status through termux-api-start to verify the app responds"},
	{Name: "Termux:Boot", CliPackage: "termux-boot",
		FDroidURL: "https://f-droid.org/en/packages/com.termux.boot/",
		SelfTest:  "Checks ~/.termux/boot exists; open the app once after install to enable it"},
	{Name: "Termux:Widget", CliPackage: "termux-widget",
		FDroidURL: "https://f-droid.org/en/packages/com.termux.widget/",
		SelfTest:  "Checks ~/.shortcuts exists (created automatically if missing)"},
	{Name: "Termux:Styling", CliPackage: "termux-styling", Optional: true,
		FDroidURL: "https://f-droid.org/en/packages/com.termux.styling/",
		SelfTest:  "None needed: this TUI already covers themes and fonts"},
}

// ShellPlugins is the plugin roster shipped by core's shell module (info only).
var ShellPlugins = []string{
	"powerlevel10k", "zsh-defer", "zsh-autosuggestions", "zsh-syntax-highlighting",
	"zsh-history-substring-search", "zsh-completions", "fzf-tab",
	"zsh-you-should-use", "zsh-autopair", "zsh-better-npm-completion",
}

// BootTemplate is a boot script template offered on Screen 7 S4.
type BootTemplate struct {
	Name    string
	Content string
}

// BootTemplates are written to ~/.termux/boot/<name>.sh with chmod +x.
var BootTemplates = []BootTemplate{
	{Name: "sshd", Content: "#!/data/data/com.termux/files/usr/bin/sh\n# Start the SSH server at boot\nsshd\n"},
	{Name: "storage", Content: "#!/data/data/com.termux/files/usr/bin/sh\n# Make sure storage symlinks exist\ntermux-setup-storage\n"},
	{Name: "blank", Content: "#!/data/data/com.termux/files/usr/bin/sh\n# New boot script: keep it short\n"},
}

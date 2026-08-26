# Termux UI — Implementation Plan

**Project codename:** `termux-ui`
**License:** MIT
**Target platform:** Termux on Android (aarch64 primary, armv7 secondary). Linux dev machines supported for building only.

## 1. Goal

Build a single-binary TUI (`termux-ui`, Go + Bubble Tea) that:

1. **Customizes everything configurable about the Termux app**: every `~/.termux/termux.properties` property, the extra-keys keyboard (with popups and macros), terminal colors (`~/.termux/colors.properties`), and terminal font (`~/.termux/font.ttf`).
2. **Installs the most common packages and apps the way `core-termux` does**: by acting as a menu front-end to the real `core` CLI (`core install|update|uninstall|reinstall|list|show`), so all installs go through core's Termux-compatible module system (pkg-based installs, wrapper bins, glibc toolchain handling). The TUI never reimplements core's install logic.
3. **Sets up system-level extras**: storage permission, Termux companion apps (API/Boot/Widget/Styling), SSH server, boot scripts, repository mirrors.

The TUI is non-destructive: every write to Termux config is preceded by a diff preview, a timestamped backup, and a confirmation; `termux-reload-settings` is invoked automatically where applicable.

## 2. Non-goals

- Reimplementing or forking core's install scripts. Core is the only installer.
- Supporting non-Termux platforms at runtime (build-only for Linux/macOS cross-compile).
- Root-requiring operations. Everything works unrooted.

## 3. Architecture

- **Stack**: Go 1.22+, `bubbletea`, `bubbles`, `lipgloss` (Charm stack). `CGO_ENABLED=0` static binary; zero runtime dependencies — runs on a fresh Termux install.
- **Termux detection**: read `TERMUX_VERSION` env var and `$PREFIX` (`/data/data/com.termux/files/usr`). If absent, print warning and exit (this tool is Termux-only, same policy as core).
- **Core bridge** (`internal/core`):
  - Detect: `command -v core`; version: `core --version`.
  - Status: parse `core list <module>` output for per-tool install state. Parser is defensive; on unrecognized output, fall back to the static module catalog (§5.2) with status `unknown`.
  - Actions: run `core install|update|uninstall|reinstall <module> [--tool ...]` via `tea.ExecProcess` so real output streams to the user (no fake spinners). On non-zero exit, show the tail of the matching log (`~/.cache/core-termux/install_<module>.log`).
  - If `core` is not installed, screen 2 shows core's official installer one-liner:
    `curl -fsSL https://raw.githubusercontent.com/DevCoreXOfficial/core-termux/main/install.sh | bash`
- **State** (`~/.cache/termux-ui/state.json`): pending restart-required changes, last update-check timestamp, cached catalog status. JSON, human-readable.
- **Distribution** (core-style):
  - `install.sh` one-liner: detects arch (`uname -m`), downloads the matching prebuilt binary from GitHub Releases to `$PREFIX/bin/termux-ui`, chmod +x.
  - `build.sh`: uses Termux's own golang (`pkg install golang`) on-device, plus `GOOS=linux GOARCH=arm64|arm` for host cross-builds. Release artifacts: `termux-ui_linux_arm64`, `termux-ui_linux_arm`.
  - Background update check once per 24 h against GitHub Releases `latest`, banner on dashboard (same UX as core).

### Repository layout

```
termux-ui/
├── main.go                  # entry, flags (--version, --no-color), TERMUX guard
├── internal/
│   ├── app/                 # root model, navigation stack, global keybindings, toast queue
│   ├── core/                # core CLI bridge: detect, version, list parser, exec
│   ├── termuxconf/          # termux.properties engine: parse/preserve comments/write/diff/backup/reload
│   ├── extrakeys/           # extra-keys model, named-key table, validation, presets, serializer
│   ├── theme/               # colors.properties writer + bundled theme registry
│   ├── font/                # font downloader/installer (direct TTF or zip+extract)
│   ├── catalog/             # static core module/tool catalog + companion apps + boot templates
│   └── ui/                  # shared widgets: menu list, form fields, toggle, radio, hex picker,
│                            #   confirm dialog, diff view, pager, toast
├── assets/themes/*.properties   # bundled color schemes
├── build.sh
├── install.sh
├── README.md
└── PLAN.md
```

## 4. Global behaviors (apply to every screen)

| Behavior | Rule |
|---|---|
| Navigation | ↑/↓ move, Enter select, Esc back, `q` quit from top level, `?` help overlay, number keys 1–7 jump to screens |
| Write safety | Any change to `~/.termux/*`: show unified diff of exactly the lines that change → `y/N` confirm → copy original to `<file>.bak.<YYYYmmdd-HHMMSS>` → write → `termux-reload-settings`. Original comments and unknown keys are always preserved verbatim. |
| Restart-required | Changes flagged restart-required are recorded in `state.json` and surfaced as a dashboard banner: "Restart Termux (force-stop from Android settings) to apply: fullscreen, cursor style, …" |
| Executing commands | Anything longer than ~1 s (pkg, core, downloads) runs via `tea.ExecProcess` with full streamed output. Ctrl+C aborts the child; TUI resumes. |
| Raw escape hatch | Every config screen offers an "Open in $EDITOR" action on the underlying file. |
| TUI self-rendering | Lip Gloss adaptive colors (light/dark terminal autodetect); `--no-color` forces monochrome. |

**Restart-required properties** (apply only after full app restart): `fullscreen`, `use-fullscreen-workaround`, `use-black-ui`, `terminal-cursor-style`, `terminal-cursor-blink-rate`. Everything else applies on `termux-reload-settings`.

## 5. Screens — full specification

### 5.1 Screen 1 — Dashboard

**Purpose**: status at a glance + quick entry points.

**Content, top to bottom**:

| Element | Source / logic |
|---|---|
| Termux version | `$TERMUX_VERSION` |
| `core` status | installed? version? path — `command -v core`, `core --version`; if missing, red line "core not installed → press I to install" |
| `termux-ui` version + update | self version vs GitHub Releases `latest` (24 h cache in `state.json`); if newer: "Update available: X (current Y)" |
| Storage permission | `test -d ~/storage` → granted/not granted |
| Pending restarts | list from `state.json` (see §4) |
| Quick actions menu | `1 Modules & Packages`, `2 Termux Settings`, `3 Extra Keys`, `4 Themes & Fonts`, `5 Shell Setup`, `6 Apps & System`, plus inline actions: `i` install core (if missing), `s` run `termux-setup-storage` (if missing), `u` self-update (download + replace binary) |

### 5.2 Screen 2 — Modules & Packages (core front-end)

**Purpose**: browse and operate core's modules/tools without memorizing flags.

**Layout**: two panes. Left: module list. Right: tools of the selected module with checkbox + status per tool. Bottom: action bar.

**Static catalog** (mirrors core 4.x; live status merged in from `core list <module>`):

| Module | Tools (flag) | Install command shape |
|---|---|---|
| `lang` | Node.js LTS (`--nodejs-lts`), Python (`--python`), Perl (`--perl`), PHP (`--php`), Rust (`--rust`), C/C++ clang (`--clang`), Go (`--golang`), Bun (`--bun`) | `core install lang --python --rust` |
| `db` | PostgreSQL (`--postgresql`), MariaDB (`--mariadb`), SQLite (`--sqlite`), MongoDB (`--mongodb`), Redis (`--redis`) | `core install db --postgresql --sqlite` |
| `ai` | OpenCode (`--opencode`), Claude Code (`--claude-code`), Codex (`--codex`), Gemini CLI (`--gemini-cli`), Qwen Code (`--qwen-code`), Ollama (`--ollama`), and the full agent list from `core list ai` | `core install ai --opencode --ollama` |
| `editor` | Neovim + NvChad bundle (module-level only) | `core install editor` |
| `dev` | gh, wget, curl, lsd, bat, proot, ncurses-utils, tmate, tmux, openssh, cloudflared, translate-shell, html2text, jq, bc, tree, fzf, imagemagick, shfmt, make, udocker, superfile `spf` (per-tool flags) | `core install dev --gh --fzf --jq` |
| `npm` | typescript, nest, prettier, live-server, localtunnel, vercel, markserv, psqlformat, ncu, ngrok, turbopack (needs glibc toolchain — shown with warning badge) | `core install npm --typescript --prettier` |
| `shell` | ZSH + powerlevel10k + plugins (module-level) | `core install shell` |
| `ui` | Termux UI components (module-level) | `core install ui` |
| `auto` | n8n (module-level) | `core install auto` |

**Tool status values**: `installed` / `not installed` / `unknown` (parser fallback).

**Actions**:

| Key | Action |
|---|---|
| Space | Toggle tool under cursor |
| `a` / `n` | Select all / none in current module |
| `i` | `core install <module> --<t1> --<t2> …` (whole module if none selected) |
| `u` | Same shape with `core update` |
| `x` | Same shape with `core uninstall` — always preceded by an explicit confirm listing exact packages to be removed |
| `r` | Same shape with `core reinstall` — confirm first |
| `d` | Docs: `core show <module> --<tool>` piped into internal pager |
| `o` | `core open <module>` (official docs in browser) |
| `L` | Open current module's log (`~/.cache/core-termux/install_<module>.log`) in pager |

**Module-level operations** (editor, shell, ui, auto) hide the per-tool pane and operate on the module as a whole.

**Error handling**: non-zero exit → show last 30 log lines + full path; "core not installed" → offer installer one-liner (§3), then re-detect.

### 5.3 Screen 3 — Termux Settings

**Purpose**: edit every documented `termux.properties` property through typed controls; never touch the file except through the engine in §4.

**Layout**: left = group list; right = form of the selected group. Footer shows modified-count; `S` saves (diff → confirm → backup → write → reload), `E` opens file in `$EDITOR`.

**Groups and fields** (control type in brackets; validation and default shown):

**G1 General**

| Property | Control | Valid values / default | Notes |
|---|---|---|---|
| `default-working-directory` | text input + directory browser | absolute path; default `$HOME` | Reject relative paths; warn if dir doesn't exist |
| `terminal-transcript-rows` | number input | 0–50000, default 2000 | Inline warning above 10000: "large buffers slow scrolling" |
| `disable-terminal-session-change-toast` | toggle | true/false, default false | |
| `hide-soft-keyboard-on-startup` | toggle | true/false, default false | |
| `soft-keyboard-toggle-behaviour` | radio | `show/hide` (default) or `enable/disable` | |
| `volume-keys` | radio | `virtual` (default) or `volume` | `volume` = keys adjust media volume, disables Ctrl/Alt emulation |
| `allow-external-apps` | toggle, guarded | true/false, default false | Requires typed confirmation: exposes Termux commands to other apps (security risk) |

**G2 Terminal & Cursor**

| Property | Control | Valid values / default | Notes |
|---|---|---|---|
| `terminal-cursor-style` | radio | `block` (default) / `bar` / `underline` | Restart-required |
| `terminal-cursor-blink-rate` | number input | `0` or `100`–`2000` (ms); 0 = no blink | Restart-required |
| `use-black-ui` | toggle | true/false, default false | Dark drawer/dialogs; auto on Android 9+ dark theme. Restart-required |
| `bell-character` | radio | `vibrate` (default) / `beep` / `ignore` | |
| `back-key` | radio | `back` (default) / `escape` | `escape` sends ESC instead of leaving |

**G3 Fullscreen & Layout**

| Property | Control | Valid values / default | Notes |
|---|---|---|---|
| `fullscreen` | toggle | true/false, default false | Restart-required |
| `use-fullscreen-workaround` | toggle | true/false, default false | Shows fixed warning: "unstable on some devices; fixes extra-keys visibility in fullscreen" |
| `terminal-margin-horizontal` | number input | 0–100 dp, default 3 | For curved screens / screen protectors |
| `terminal-margin-vertical` | number input | 0–100 dp, default 0 | |

**G4 Keyboard Workarounds**

| Property | Control | Valid values / default | Notes |
|---|---|---|---|
| `enforce-char-based-input` | toggle | true/false, default false | Help text: "fixes keyboards (e.g. Samsung) that only commit text on Enter" |
| `ctrl-space-workaround` | toggle + danger note | true/false, default false | Fixed warning: "breaks Ctrl+Space on devices where it already works" |

**G5 Hardware Session Shortcuts**

| Property | Control | Example/default |
|---|---|---|
| `shortcut.create-session` | text input | `ctrl + t` |
| `shortcut.next-session` | text input | `ctrl + 2` |
| `shortcut.previous-session` | text input | `ctrl + 1` |
| `shortcut.rename-session` | text input | `ctrl + n` |
| `disable-hardware-keyboard-shortcuts` | toggle | true/false, default false |

Shortcut inputs validated against the `ctrl + <key>` form; invalid entries block save with field highlighted.

**G6 Info (read-only)**: volume-key shortcut cheat sheet (Vol-down = Ctrl; Vol-up = special keys: E=Esc, T=Tab, 1–0=F1–F10, W/A/S/D=arrows, B/F/X=Alt combos, L=`|`, H=`~`, U=`_`, P/N=PGUP/PGDN, Q=show extra keys, V=volume control) and link to open the raw file.

**Engine rules**: Java `.properties` semantics (`key=value`, `#` comments, backslash line continuations). Values shown in the TUI reflect effective state = file value if present, else built-in default; properties absent from the file are written only when the user changes them from default, and a `reset to default` action removes the key instead of writing the default.

### 5.4 Screen 4 — Extra Keys Builder

**Purpose**: build `extra-keys` layouts visually, including popups and macros, with all Termux constraints enforced before save.

**Layout**: top = live preview (the key rows rendered as buttons using the current `extra-keys-style` symbols); middle = grid editor; bottom = palette + actions.

**Grid editor**: cursor moves with arrows over N rows × M keys; each cell shows its label, `P` badge if it has a popup, `M` badge if it's a macro. Enter opens the cell editor.

**Cell editor** fields:

| Field | Options |
|---|---|
| Type | `literal character` / `named key` / `macro` |
| Value | literal: any single printable char except `\` (rejected with hint "use BACKSLASH"); named key: pick from palette; macro: free text of space-separated tokens, e.g. `CTRL f d`, `:q\n`, `ALT j` |
| Display label | optional, macros only (e.g. `A-j`) |
| Popup | none / simple key (palette or literal) / macro + display label |

**Named-key palette** (complete list, grouped in the UI):
- Modifiers: `CTRL`, `ALT`, `FN`, `SHIFT` — **each may appear at most once in the entire layout**; duplicates are blocked at save and highlighted in red
- Navigation/editing: `SCROLL`, `SPACE`, `ESC`, `TAB`, `HOME`, `END`, `PGUP`, `PGDN`, `INS`, `DEL`, `BKSP`, `UP`, `DOWN`, `LEFT`, `RIGHT`, `ENTER`
- Symbols: `BACKSLASH`, `QUOTE`, `APOSTROPHE`
- Function keys: `F1` … `F12`
- Actions: `KEYBOARD` (hide soft keyboard), `DRAWER` (open app drawer)

**Layout constraints enforced**: modifier uniqueness (above); no literal backslash; max 6 rows × 12 columns (UI cap for usability); every row must have ≥ 1 key; popups require Termux v0.95+ (checked via `$TERMUX_VERSION`, warning only).

**Presets** (exact serialized forms shipped in `catalog/`):

| Preset | Definition |
|---|---|
| Default (2 rows) | `[['ESC','/',{key: '-', popup: '|'},'HOME','UP','END','PGUP'], ['TAB','CTRL','ALT','LEFT','DOWN','RIGHT','PGDN']]` |
| Single row | `[[ESC, TAB, CTRL, ALT, {key: '-', popup: '|'}, DOWN, UP]]` |
| tmux | Termux-wiki tmux layout (ESC/CTRL/ALT with tmux-prefix macros `CTRL f …`, arrows with HOME/PGDN/PGUP/END popups, `ALT j`/`ALT g` macro key, KEYBOARD with `CTRL d` exit popup) |
| Vim | termux-tools vim template (`:q\n` QuickExit on ESC, `\\\\` popup on `/`, `:w`/`:wq\n` on TAB/CTRL, CTRL+HOME/END/arrow popups, `(`→`{`, `#`→`$`, `)`→`}`) |

**Companion fields on the same screen**:

| Property | Control | Values |
|---|---|---|
| `extra-keys-style` | radio | `default` / `arrows-only` / `arrows-all` / `all` / `none` |
| `extra-keys-text-all-caps` | toggle | true (default) / false |

**Serialization**: written as a multiline backslash-continued value (Termux canonical form). After save+reload, toast: "Show/hide the row: long-press the keyboard button or Vol-up+Q".

### 5.5 Screen 5 — Themes & Fonts

**Purpose**: install color schemes and Nerd Fonts without manual file copying.

**Tabs**: `Colors` | `Font`.

**Colors tab**:

| Element | Behavior |
|---|---|
| Gallery grid | Cards: theme name + 8 foreground swatch pairs rendered from the theme's ANSI palette. Bundled: Nord, Dracula, Gruvbox Dark, Catppuccin Macchiato, Tokyo Night, Solarized Dark, Solarized Light, One Dark, Monokai, Rosé Pine (each an `assets/themes/<name>.properties` with `color0`–`color15`, `foreground`, `background`, `cursor`) |
| Apply | Enter → live preview pane repaints with theme → confirm → backup current `~/.termux/colors.properties` → write → `termux-reload-settings` |
| Custom editor | 16 ANSI colors + `foreground` + `background` + `cursor`, each editable via hex input (`#RRGGBB` validated) or a 256-color palette browser; "current" loads the live file |
| Import | Load any local `.properties` file through the file browser |
| Reset | Delete `colors.properties` (after confirm) to return to app defaults |

**Font tab**:

| Element | Behavior |
|---|---|
| Font list | MesloLGS NF Regular/Bold/Italic/BoldItalic (recommended for powerlevel10k — direct TTF URLs from `romkatv/powerlevel10k-media`), JetBrainsMono NF, FiraCode NF, CaskaydiaCove NF, Hack NF (from `ryanoasis/nerd-fonts` release zips — requires `unzip`, auto-installed via `pkg install unzip -y` if missing) |
| Install | Download → write `~/.termux/font.ttf` → `termux-reload-settings` → note "if not applied immediately, force-stop Termux" |
| Local file | Pick any local `.ttf` via file browser and install the same way |
| Remove font | Delete `~/.termux/font.ttf` after confirm (returns to built-in font) |

### 5.6 Screen 6 — Shell Setup

**Purpose**: set up the core-managed ZSH environment and switch default shell.

**Content**:

| Element | Source / behavior |
|---|---|
| Current shell | `$SHELL` basename (zsh/bash/other) |
| ZSH stack status | from `core list shell` — installed/not |
| Plugin roster (info, from core's shell module) | powerlevel10k, zsh-defer, zsh-autosuggestions, zsh-syntax-highlighting, zsh-history-substring-search, zsh-completions, fzf-tab, zsh-you-should-use, zsh-autopair, zsh-better-npm-completion |
| Install stack | `core install shell` via exec; on success: "Restart Termux to apply" + recorded as restart-pending |
| Update plugins | `core update shell` |
| Reinstall stack | `core reinstall shell` (confirm) |
| Switch default shell | `chsh` presented as radio (bash / zsh) → runs `chsh -s <shell>` via exec; validates the target exists in `$PREFIX/bin` first |
| Reconfigure prompt | `p10k configure` via exec (only when powerlevel10k detected) |

### 5.7 Screen 7 — Apps & System

**Purpose**: everything around packages: permissions, companion apps, services, boot.

**Sections**:

**S1 Storage**: status (`~/storage` exists?) → run `termux-setup-storage` (exec; Android permission dialog appears). Note: command is interactive with Android UI — user is told to accept the dialog, then press Enter to re-check.

**S2 Companion apps** (each row: CLI package status + app install action):

| App | CLI part (pkg) | App part | Self-test |
|---|---|---|---|
| Termux:API | `termux-api` | open F-Droid page with `termux-open-url https://f-droid.org/en/packages/com.termux.api/` | `termux-api-start` then `termux-battery-status` — output shown |
| Termux:Boot | `termux-boot` | `termux-open-url https://f-droid.org/en/packages/com.termux.boot/` | check `~/.termux/boot` exists; instructions to open the app once to enable |
| Termux:Widget | `termux-widget` | `termux-open-url https://f-droid.org/en/packages/com.termux.widget/` | check `~/.shortcuts` exists, create if missing |
| Termux:Styling | `termux-styling` | `termux-open-url https://f-droid.org/en/packages/com.termux.styling/` | none (info: duplicates this TUI's theme/font features — label it "optional") |

Note shown once: companion apps must come from **F-Droid** (Play Store builds are incompatible/inactive).

**S3 SSH server wizard** (step list, each step runnable/re-runnable):

1. `pkg install openssh -y`
2. Set password — masked input → `chpasswd`-equivalent via `passwd` exec (or writes via `chpasswd` if available; fallback: instruct manual `passwd`)
3. Start: `sshd` — status check: `pgrep sshd`
4. Info panel: port **8020**, login `ssh -p 8020 <user>@<phone-ip>` (IP from `ifconfig`/`ip addr` parse), warning "sshd dies on reboot — add to boot scripts (S4)"
5. Stop: `pkill sshd`

**S4 Boot scripts manager** (requires Termux:Boot; shows hint if missing):

| Element | Behavior |
|---|---|
| Script list | `~/.termux/boot/*.sh` with name + executable bit |
| Create | Name input + template picker: `sshd` (start SSH), `storage` (termux-setup-storage), `blank` → opens in `$EDITOR`; saved `chmod +x` |
| Edit / Delete | `$EDITOR` / confirm + delete |
| Note | Scripts run at boot; keep them short; open the Termux:Boot app once after install |

**S5 System maintenance**:

| Action | Command |
|---|---|
| Change package mirrors (interactive) | `termux-change-repo` via exec |
| Upgrade all packages | `pkg upgrade -y` via exec (confirm first) |
| System info | `termux-info` piped to pager |

## 6. Data formats

| File | Format | Owner |
|---|---|---|
| `~/.termux/termux.properties` | Java properties; comments preserved | engine in `internal/termuxconf` |
| `~/.termux/colors.properties` | `color0`–`color15`, `foreground`, `background`, `cursor` as `#RRGGBB` | `internal/theme` |
| `~/.termux/font.ttf` | binary TTF | `internal/font` |
| `~/.cache/termux-ui/state.json` | restart-pending list, update-check timestamp | `internal/app` |
| Backups | `<orig>.bak.<YYYYmmdd-HHMMSS>` next to the original | all writers |

## 7. Milestones

| # | Scope | Acceptance criteria |
|---|---|---|
| M1 | Scaffold + core bridge + Screen 2 + Dashboard | On fresh Termux: `termux-ui` runs, detects/installs core, browses all 9 modules, installs `dev --gh --fzf` with streamed output and correct status refresh |
| M2 | Screen 3 (all groups) + Screen 4 | Every property in §5.3/§5.4 editable, validated, saved with diff+backup+reload; each preset loads, edits, and serializes byte-identical to its shipped form; modifier-duplicate and backslash rules block save |
| M3 | Screen 5 | Theme apply changes live colors; MesloLGS NF installs and renders p10k glyphs; custom hex editor round-trips the file |
| M4 | Screens 6 + 7 | Shell switch persists after restart; SSH wizard reaches login from a remote host; boot script survives reboot and starts sshd |
| M5 | build.sh + install.sh + README + first GitHub Release | One-liner installs prebuilt arm64 binary on a fresh Termux and `termux-ui --version` matches the release tag |

## 8. Conventions

- Go packages under `internal/` only; no external state outside `~/.termux`, `~/.cache/termux-ui`.
- All user-facing strings in clear technical English; destructive actions named explicitly ("Delete font file", "Uninstall 3 tools").
- Version: CalVer `v0.YYMMDD` + short commit hash in releases.

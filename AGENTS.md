# DLP-Go

## Goal
Build a Go CLI TUI app using Bubble Tea that auto-manages yt-dlp (stable/nightly), lets the user paste a URL, select quality presets, download videos with progress, and check for yt-dlp updates.

## Constraints
- Go + Bubble Tea + Bubbles + Lipgloss
- Directory: binary stays in project root, `yt-dlp/` for binaries+config, `Descargas/` for videos
- Auto-download yt-dlp nightly if not found (from GitHub releases)
- Settings screen via `C` key: choose stable/nightly, check for updates via GitHub API, download newer version
- Cross-platform (Linux + Windows)
- 5 quality presets (1080p, 720p, 480p, audio MP3, best available)

## State Machine
- `checkingState` → startup: verify yt-dlp exists
- `downloadingBinState` → auto-download yt-dlp if missing
- `inputState` → main screen: paste URL
- `loadingState` → fetch video info via `--dump-json`
- `selectState` → pick quality preset
- `downloadingState` → subprocess with progress bar
- `doneState` → error/success message
- `settingsState` → configure source, check updates
- `checkingUpdateState` → GitHub API version lookup
- `updatingState` → download new yt-dlp binary

## Key Architecture
- **ytdlp_types.go**: types, config load/save, GitHub API, version checking, file download helpers
- **tui.go**: Bubble Tea model, update loop, views (7 screens), styles
- **ytdlp.go**: yt-dlp subprocess manager (context cancellation, channel-based progress streaming)
- **main.go**: entry point, creates `tea.NewProgram` with alt screen

## Config
- `<app_dir>/yt-dlp/config.json` → `{"source":"nightly"}` (or `"stable"`)
- `<app_dir>/yt-dlp/nightly/yt-dlp` — nightly binary
- `<app_dir>/yt-dlp/stable/yt-dlp` — stable binary
- `<app_dir>/Descargas/` — downloaded videos

## Commands
- `go run .` — run in dev
- `go build -o DLP-Go .` — build binary
- `go vet ./...` — lint
- `GOOS=windows GOARCH=amd64 go build -o DLP-Go.exe .` — cross-compile Windows

## Key Bindings
- `Enter` — confirm/continue
- `C` — settings (toggle)
- `1`/`2` — select stable/nightly source
- `U` — check for updates (in settings)
- `D` — download/update (in settings, when update available)
- `↑`/`↓` or `k`/`j` — navigate quality list
- `Ctrl+C` or `q` — quit (also cancels download)
- `Esc` — back from settings

## urls
- yt-dlp nightly: `https://github.com/yt-dlp/yt-dlp-nightly-builds/releases/latest/download/yt-dlp`
- yt-dlp stable: `https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp`
- GitHub API nightly: `https://api.github.com/repos/yt-dlp/yt-dlp-nightly-builds/releases/latest`
- GitHub API stable: `https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest`

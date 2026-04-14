# sshtui

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/diegovrocha/sshtui)](https://github.com/diegovrocha/sshtui/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/diegovrocha/sshtui)](https://go.dev/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/diegovrocha/sshtui/pulls)
[![Go Report Card](https://goreportcard.com/badge/github.com/diegovrocha/sshtui)](https://goreportcard.com/report/github.com/diegovrocha/sshtui)

```
  _ _     _____ _   _ ___
 ___ ___| |__ |_   _| | | |_ _|   SSH + TUI
/ __/ __| '_ \  | | | | | || |   Inspect and generate SSH keys
\__ \__ \ | | | | | | |_| || |   and certificates.
|___/___/_| |_| |_|  \___/|___|   https://github.com/diegovrocha/sshtui
```

TUI for inspecting and generating SSH keys and certificates. Uses `ssh-keygen` under the hood.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Single binary, only dep: `openssh-client`.

**Contributions welcome!** See [CONTRIBUTING.md](CONTRIBUTING.md).

## Requirements

- **openssh-client** — pre-installed on macOS and most Linux distributions. Needs `ssh-keygen` in `$PATH`.

## Install

### Quick install (macOS/Linux)

```bash
curl -sSLf https://raw.githubusercontent.com/diegovrocha/sshtui/main/install.sh | sh
```

### Manual download

Download the binary for your platform from [Releases](https://github.com/diegovrocha/sshtui/releases):

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `sshtui_darwin_arm64.tar.gz` |
| macOS (Intel) | `sshtui_darwin_amd64.tar.gz` |
| Linux (amd64) | `sshtui_linux_amd64.tar.gz` |
| Linux (arm64) | `sshtui_linux_arm64.tar.gz` |
| Windows (amd64) | `sshtui_windows_amd64.zip` |

Extract and move to your PATH:

```bash
tar -xzf sshtui_<os>_<arch>.tar.gz
sudo mv sshtui /usr/local/bin/
```

### From source

Requires [Go 1.22+](https://go.dev/dl/):

```bash
git clone https://github.com/diegovrocha/sshtui.git
cd sshtui
make install    # builds and copies to /usr/local/bin
```

Other make targets:

```bash
make build           # build binary locally (with version injected via ldflags)
make test            # run Go tests
make vet             # run go vet
make check           # vet + test
make uninstall       # remove from /usr/local/bin

# Release (maintainers only)
make release-auto    # detect bump from commit messages (recommended)
make release-patch   # bug fix: v1.3.0 → v1.3.1
make release-minor   # new feature: v1.3.0 → v1.4.0
make release-major   # breaking change: v1.3.0 → v2.0.0
make release VERSION=1.5.0  # explicit version
```

Each `release-*` target runs `go vet`, tests, tags and pushes. GitHub Actions then builds and publishes the release automatically.

`release-auto` inspects the commit messages since the last tag and picks the bump kind using [conventional commits](https://www.conventionalcommits.org/):

| Commit prefix | Bump |
|---|---|
| `feat!:`, `fix!:`, or `BREAKING CHANGE:` in body | **major** |
| `feat:`, `feat(scope):` | **minor** |
| anything else (`fix:`, `docs:`, `refactor:`, etc.) | **patch** |

## Features

### Inspect
- **SSH key** — public or private key: fingerprint, type, size, comment, encrypted flag
- **SSH cert** — principals, validity, signing CA, key ID, extensions

### Generate
- **SSH key** — Ed25519 / RSA / ECDSA / DSA, configurable bits / curve, optional passphrase, comment

### Utilities
- **History** — log of all operations stored in `~/.sshtui/history.log`, viewable from the menu
- **Update** — in-app download and replace of the binary. Shows scrollable GitHub release notes before installing, then auto-restarts sshtui with the new version (3-second countdown, press `r` to restart immediately or `c` to cancel). Also auto-detects new releases on launch and shows a notice in the banner
- **Quit**

## Navigation

Press `?` on any screen to see a contextual help overlay listing the keys that screen understands.

### General
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate menu and lists |
| `Enter` | Select / Confirm / Open details |
| `Esc` | Back to previous screen |
| `q` | Quit (main menu only) |
| `Ctrl+C` | Quit from anywhere |
| `/` | Fuzzy search filter in main menu |
| `?` | Toggle contextual help |

### File picker
| Key | Action |
|-----|--------|
| `→` or `Enter` | Enter highlighted folder |
| `←` | Go to parent directory |
| Type | Filter files by name |

### Inspect results
| Key | Action |
|-----|--------|
| `f` | Toggle full view (extra fields) |
| `y` | Copy details to clipboard |
| `s` | Save details as `.txt` |
| `n` | Inspect another key / cert |
| `↑/↓` | Scroll long output |

### Update
| Key | Action |
|-----|--------|
| `↑/↓` | Scroll changelog |
| `Enter` | Install update (on confirm step) / Restart now (on success) |
| `r` | Restart now after update |
| `c` | Cancel auto-restart |

## Docker (test on Linux)

A `Dockerfile.test` is provided to try sshtui on Linux without installing anything locally:

```bash
docker build -t sshtui-test -f Dockerfile.test .
docker run -it --rm -v $(pwd):/keys sshtui-test
```

The container mounts your current directory as `/keys` so sshtui can access local key files. Uses `debian:stable-slim` and downloads the latest released binary automatically.

## Theme

sshtui auto-detects light / dark terminals via the `$COLORFGBG` environment variable and picks appropriate colors. To override detection:

```bash
SSHTUI_THEME=light sshtui
SSHTUI_THEME=dark  sshtui
```

| Variable | Values | Effect |
|----------|--------|--------|
| `SSHTUI_THEME` | `light`, `dark` | Force theme (overrides autodetection) |
| `COLORFGBG` | auto | Read from terminal for autodetection |

## Screenshots / demos

TODO — add screenshots and an asciinema demo. For now, run `sshtui` to see it in action.

## License

[MIT](LICENSE) - Diêgo Vieira Rocha

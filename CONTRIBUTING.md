# Contributing to sshtui

Thanks for your interest in contributing! All contributions are welcome — bug fixes, new features, documentation improvements, tests, or ideas.

## Quick start

```bash
git clone https://github.com/diegovrocha/sshtui.git
cd sshtui
go build -o sshtui ./cmd/sshtui
./sshtui
```

## Requirements

- [Go 1.22+](https://go.dev/dl/)
- `openssh-client` installed and `ssh-keygen` in `$PATH`

## Making changes

1. Fork the repo and create a branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes. Keep commits small and focused.

3. Run tests:
   ```bash
   make test
   ```

4. Build locally and try it out:
   ```bash
   make build
   ./sshtui
   ```

5. Commit with a clear message:
   ```
   feat: add something
   fix: resolve bug X
   docs: update README
   test: add coverage for Y
   ```

6. Push to your fork and open a Pull Request.

## Code guidelines

- **Tests**: add tests for new features or bug fixes in the respective `*_test.go` file
- **UI consistency**: follow the style in `internal/ui/` (lipgloss styles, ANSI-aware rendering)
- **Sub-models**: new operations should be Bubble Tea models under `internal/<feature>/`
- **File picker**: reuse `ui.FilePicker` with the appropriate constructor for the operation (key picker, cert picker, etc.)
- **Stdout discipline**: functions captured via `$(...)` or subprocess pipes must send only the return value to stdout; all UI messages go to stderr
- **English only**: all code, comments, and UI strings are in English

## Project structure

```
sshtui/
├── cmd/sshtui/main.go            # entrypoint
├── internal/
│   ├── menu/                     # main menu
│   ├── inspect/                  # SSH key / cert inspection
│   ├── generate/                 # SSH key generation
│   └── ui/                       # shared components (styles, filepicker, stats)
├── .github/workflows/            # CI (test + release)
├── .goreleaser.yaml              # cross-platform release config
└── install.sh                    # one-line installer
```

## Adding a new menu option

1. Create your model in `internal/<feature>/<name>.go` implementing `tea.Model`
2. Add the menu entry in `internal/menu/menu.go` (`items` slice)
3. Route the action in `handleAction()`
4. Add tests in `<name>_test.go`

## Reporting bugs

Open an issue at https://github.com/diegovrocha/sshtui/issues with:

- OS and terminal (macOS/Linux, iTerm/Alacritty/etc.)
- `sshtui` version (shown in banner)
- `ssh-keygen -V` / `ssh -V` output
- Steps to reproduce
- Expected vs actual behavior

## Code of conduct

Be respectful. We're all here to learn and make useful tools together.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

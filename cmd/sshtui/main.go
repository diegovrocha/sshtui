package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/diegovrocha/sshtui/internal/menu"
	"github.com/diegovrocha/sshtui/internal/update"
)

func main() {
	p := tea.NewProgram(menu.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If the user just updated and requested a restart, re-exec this binary.
	if update.RestartRequested {
		if err := restart(); err != nil {
			fmt.Fprintf(os.Stderr, "Auto-restart failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "Please run sshtui again manually.")
			os.Exit(1)
		}
	}
}

// restart replaces the current process with a fresh instance of the binary.
// On Unix, syscall.Exec swaps the process image (clean, no orphaned PID).
// On Windows, we spawn a new process and exit.
func restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := os.Args
	env := os.Environ()

	if runtime.GOOS == "windows" {
		cmd := exec.Command(exe, args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			return err
		}
		os.Exit(0)
		return nil
	}

	// Unix: replace process image
	return syscall.Exec(exe, args, env)
}

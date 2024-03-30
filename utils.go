package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/charmbracelet/lipgloss"
	"github.com/mitchellh/go-ps"
)

func copyFile(src, dst string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file '%s': %w", src, err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", dst, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy data from '%s' to '%s': %w", src, dst, err)
	}
	if err := os.Chtimes(dst, stat.ModTime(), stat.ModTime()); err != nil {
		return fmt.Errorf("failed to copy modification time from '%s' to '%s': %w", src, dst, err)
	}
	return nil
}

func displayBox(color string, format string, a ...any) {
	fmt.Println()
	fmt.Println(
		lipgloss.NewStyle().
			Width(79).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color)).
			Foreground(lipgloss.Color(color)).
			Padding(1, 2).
			Render(fmt.Sprintf(format, a...)),
	)
	fmt.Println()
}

func displayNotice(format string, a ...any) {
	displayBox("12", format, a...) // blue
}

func displayWarning(format string, a ...any) {
	displayBox("9", format, a...) // red
}

func openDirectoryInExplorer(dir string) error {
	cmd := exec.Command("explorer", dir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open directory in explorer: %w", err)
	}
	return nil
}

func processIsRunning(name string) (bool, error) {
	procs, err := ps.Processes()
	if err != nil {
		return false, fmt.Errorf("failed to list running processes: %w", err)
	}
	for _, proc := range procs {
		if proc.Executable() == name {
			return true, nil
		}
	}
	return false, nil
}

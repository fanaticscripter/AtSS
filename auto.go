package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/zmwangx/debounce"

	"github.com/fanaticscripter/AtSS/log"
)

type autoBackupDisplayMsg string

type autoBackupTeaModel struct {
	displayMessagesCh <-chan string
	displayMessages   []autoBackupDisplayMsg
	spinner           spinner.Model
	quitting          bool
}

func autoBackupInitialModel(displayMessagesCh <-chan string) autoBackupTeaModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return autoBackupTeaModel{
		displayMessagesCh: displayMessagesCh,
		spinner:           s,
	}
}

func displayMessageListenCmd(ch <-chan string) tea.Cmd {
	// The command waits for and returns the next message from the channel.
	return func() tea.Msg {
		return autoBackupDisplayMsg(<-ch)
	}
}

func (m autoBackupTeaModel) Init() tea.Cmd {
	return tea.Batch(displayMessageListenCmd(m.displayMessagesCh), m.spinner.Tick)
}

func (m autoBackupTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}

	case autoBackupDisplayMsg:
		m.displayMessages = append(m.displayMessages, msg)
		return m, displayMessageListenCmd(m.displayMessagesCh)

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m autoBackupTeaModel) View() string {
	var sb strings.Builder
	for _, m := range m.displayMessages {
		sb.WriteString(string(m))
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "\n%s Waiting for updated save...\n", m.spinner.View())
	if m.quitting {
		sb.WriteString("\n")
	}
	return sb.String()
}

func startAutoBackups() {
	displayNotice("This program will now watch for changes to the game save, and automatically back up when a change is detected. " +
		"There is a few seconds' delay for each update.\n\n" +
		"Please keep this window open.")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create filesystem watcher: %s", err)
		return
	}
	defer watcher.Close()

	displayMessagesCh := make(chan string, 1)
	// We debounce the backup operation, because sometimes multiple save files
	// need to be updated, and even when only a single one changes, it may not
	// be written atomically, so multiple write events can fire in quick
	// succession.
	performBackup, _ := debounce.Debounce(func() {
		backup, err := createBackup(BackupMetadata{
			Note: "auto backup",
		})
		if err != nil {
			now := time.Now()
			displayMessagesCh <- lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
				fmt.Sprintf("[%s] failed to create auto backup: %s", now.Format("2006-01-02 15:04:05"), err),
			)
		} else {
			displayMessagesCh <- fmt.Sprintf("created backup: %s", backup)
		}
	}, 5*time.Second, debounce.WithMaxWait(30*time.Second))
	performBackup() // Perform a backup upon startup
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					performBackup()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Errorf("filesystem watcher error: %s", err)
			}
		}
	}()

	err = watcher.Add(_savesDirectory)
	if err != nil {
		log.Fatalf("failed to watch saves directory '%s': %s", _savesDirectory, err)
	}

	p := tea.NewProgram(autoBackupInitialModel(displayMessagesCh))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/fanaticscripter/AtSS/log"
)

var _rootCmd = &cobra.Command{
	Use:   "AtSS",
	Short: "Against the Storm Save Scummer",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var action string
		// Use reverse on the selected option, since the default highlight color
		// is overridden.
		theme := huh.ThemeCharm()
		theme.Focused.SelectedOption.Reverse(true)
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Choose an action").
					Options(
						huh.NewOption(colored(_green, "Save current state"), "save"),
						huh.NewOption(colored(_green, "Save current and future states automatically"), "autosave"),
						huh.NewOption(colored(_blue, "Restore previously saved state"), "restore"),
						huh.NewOption(colored(_red, "Delete previously saved states"), "delete"),
						huh.NewOption(colored(_yellow, "Open saves directory"), "open"),
					).
					Value(&action),
			),
		).WithTheme(theme)
		if err := form.Run(); err != nil {
			log.Fatal(err)
		}
		switch action {
		case "save":
			if err := createBackupInteractive(); err != nil {
				log.Fatal(err)
			}
		case "autosave":
			startAutoBackups()
		case "restore":
			if err := restoreBackupInteractive(); err != nil {
				log.Fatal(err)
			}
			log.Exit(0)
		case "delete":
			if err := deleteBackupsInteractive(); err != nil {
				log.Fatal(err)
			}
			log.Exit(0)
		case "open":
			openSavesDirectory()
		}
	},
}

var _saveCmdNote string

var _saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save the current state",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		noteFlag := cmd.Flags().Lookup("note")
		if noteFlag.Changed {
			// --note flag is set, activate non-interactive mode.
			backup, err := createBackup(BackupMetadata{
				Note: _saveCmdNote,
			})
			if err != nil {
				log.Fatal(err)
			}
			log.Infof("created backup %s", backup.Dir)
		} else {
			if err := createBackupInteractive(); err != nil {
				log.Fatal(err)
			}
		}
	},
}

var _autoSaveCmd = &cobra.Command{
	Use:   "autosave",
	Short: "Save current and future states automatically",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		startAutoBackups()
	},
}

var _restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a previously saved state",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := restoreBackupInteractive(); err != nil {
			log.Fatal(err)
		}
		log.Exit(0)
	},
}

var _deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete previsouly saved states",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := deleteBackupsInteractive(); err != nil {
			log.Fatal(err)
		}
		log.Exit(0)
	},
}

var _openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the saves directory",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		openSavesDirectory()
	},
}

func init() {
	// Allow program to be launched from explorer.exe directly, instead of being
	// trapped by cobra.
	cobra.MousetrapHelpText = ""
}

func main() {
	_saveCmd.Flags().StringVarP(&_saveCmdNote, "note", "n", "", "note to attach to the save, may be empty; the save is created non-interactively if this flag is set")
	_rootCmd.AddCommand(_saveCmd, _autoSaveCmd, _restoreCmd, _deleteCmd, _openCmd)

	if err := _rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

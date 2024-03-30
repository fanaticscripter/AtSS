package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/fanaticscripter/AtSS/log"
)

func createBackupInteractive() error {
	var notice string
	lastModified, _, err := getSaveAge()
	if err != nil {
		log.Warnf("failed to determine the age of your current game save: %s", err)
	} else {
		reltime := humanize.RelTime(lastModified, time.Now(), "ago", "")
		notice += fmt.Sprintf("Your current game save is from %s.\n\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Underline(true).Render(reltime)) // Highlight in green
	}
	notice += `If you haven't done so yet, "Save and Quit" to main menu before you proceed, ` +
		`which will create an up-to-date game save. ` +
		`You don't have to exit the game.`
	displayNotice(notice)

	var note string
	inputGroup := huh.NewGroup(
		huh.NewInput().
			Title("Optional note").
			Description("Will be shown when you need to choose a saved state to restore.").
			Value(&note),
	)
	form := huh.NewForm(inputGroup)
	if err := form.Run(); err != nil {
		return fmt.Errorf("failed to get note from user: %w", err)
	}
	// Keep the input form on screen.
	fmt.Println(inputGroup.WithShowHelp(false).View())

	backup, err := createBackup(BackupMetadata{
		Note: note,
	})
	if err != nil {
		return err
	}
	log.Infof("created backup '%s'", backup.Dir)
	return nil
}

func restoreBackupInteractive() error {
	backups, err := getBackups()
	if err != nil {
		return err
	}
	var options []huh.Option[Backup]
	var hasOverwritten bool
	for _, b := range backups {
		options = append(options, huh.NewOption(b.String(), b))
		if b.Metadata.IsOverwritten {
			hasOverwritten = true
		}
	}

	displayNotice("Please quit to main menu before you proceed.\n\n" +
		"You don't need to exit the game if you're going back to an earlier point of the same settlement, " +
		"but you do need to exit the game first if you're going back to the map to reroll a biome, or going back to an earlier state of the map.")

	var backup Backup
	for {
		var description string
		if hasOverwritten {
			description = "The backup marked as [overwritten] was auto created during your last restore."
		}
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[Backup]().
					Title("Choose a backup to restore").
					Description(description).
					Options(options...).
					Value(&backup),
			),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("failed to get user selection: %w", err)
		}
		// Confirm.
		//
		// huh doesn't seem to support defaulting to yes, so we have to reverse
		// the yes/no as a workaround.
		var chooseAgain bool
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Restore backup '%s'?", backup)).
					Affirmative("No").
					Negative("Yes").
					Value(&chooseAgain),
			),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}
		if !chooseAgain {
			break
		}
	}

	_, err = restoreBackup(backup)
	if errors.Is(err, _errGameIsRunningRestoreRefused) {
		displayWarning("You need to quit the game before performing this restore, or the changes won't take full effect.\n\n" +
			"Please quit the game (quitting to main menu isn't enough) and try the restore again.")
	}
	if err != nil {
		return err
	}
	return nil
}

func deleteBackupsInteractive() error {
	backups, err := getBackups()
	if err != nil {
		return err
	}
	var options []huh.Option[Backup]
	for _, b := range backups {
		if b.Metadata.IsOverwritten {
			// There's only one copy of this auto backup, no point deleting it.
			continue
		}
		options = append(options, huh.NewOption(b.String(), b))
	}
	var toDelete []Backup
	for {
		selectGroup := huh.NewGroup(
			huh.NewMultiSelect[Backup]().
				Title("Choose backups to delete").
				Options(options...).
				Value(&toDelete).
				Validate(func(s []Backup) error {
					if len(s) == 0 {
						return fmt.Errorf("nothing is selected")
					}
					return nil
				}),
		)
		form := huh.NewForm(selectGroup)
		if err := form.Run(); err != nil {
			return fmt.Errorf("failed to get user selection: %w", err)
		}
		// Keep the selection displayed.
		fmt.Println(selectGroup.WithShowHelp(false).View())

		// Confirm.
		var proceed bool
		form = huh.NewForm(
			huh.NewGroup(huh.NewConfirm().Title("Are you sure?").Value(&proceed)),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}
		if proceed {
			break
		}
	}
	for _, b := range toDelete {
		if err := os.RemoveAll(b.Dir); err != nil {
			log.Errorf("failed to delete backup '%s': %s", b.Dir, err)
		} else {
			log.Infof("deleted backup '%s'", b.Dir)
		}
	}
	return nil
}

func openSavesDirectory() {
	if err := openDirectoryInExplorer(_eremiteGamesRootDirectory); err != nil {
		log.Error(err)
	}
}

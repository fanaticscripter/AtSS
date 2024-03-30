package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fanaticscripter/AtSS/log"
)

const (
	_againstTheStormExecutable = "Against the Storm.exe"

	_backupRootDirname        = "Against the Storm - AtSS Backups"
	_metadataFilename         = "atss.json"
	_backupDirnameFormat      = "Bak.2006-01-02_15.04.05"
	_overwrittenBackupDirname = "Bak.overwritten"
)

var _backupsDirectory string

var _errGameIsRunningRestoreRefused = fmt.Errorf(
	"game is running, refusing to restore backup since the changes won't apply properly",
)

type Backup struct {
	Metadata BackupMetadata
	Dir      string
}

type BackupMetadata struct {
	CreatedAt     time.Time `json:"createdAt"`
	IsOverwritten bool      `json:"isOverwritten"` // Whether this is an automatic backup created on restore
	Note          string    `json:"note"`
}

func (b Backup) String() string {
	s := b.Metadata.CreatedAt.Format("2006-01-02 15:04:05")
	if b.Metadata.Note != "" {
		s += fmt.Sprintf(" (%s)", b.Metadata.Note)
	}
	if b.Metadata.IsOverwritten {
		s = "[overwritten] " + s
	}
	return s
}

func createBackup(metadata BackupMetadata) (backup Backup, err error) {
	pattern := filepath.Join(_savesDirectory, "*.save")
	saveFiles, _ := filepath.Glob(pattern)
	if len(saveFiles) == 0 {
		err = fmt.Errorf("failed to find save files '%s'", pattern)
		return
	}
	// Warn if one or more expected save files are missing.
	for _, expected := range _expectedSaveFiles {
		found := false
		for _, f := range saveFiles {
			if filepath.Base(f) == expected {
				found = true
				break
			}
		}
		if !found {
			log.Warnf("expected save file '%s' not found in '%s'", expected, _savesDirectory)
		}
	}

	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = time.Now().Truncate(time.Second)
	}
	dirname := metadata.CreatedAt.Format(_backupDirnameFormat)
	if metadata.IsOverwritten {
		dirname = _overwrittenBackupDirname
	}
	backup = Backup{
		Metadata: metadata,
		Dir:      filepath.Join(_backupsDirectory, dirname),
	}

	if metadata.IsOverwritten {
		if err = os.RemoveAll(backup.Dir); err != nil {
			err = fmt.Errorf("failed to remove existing overwritten backup directory '%s': %w", backup.Dir, err)
			return
		}
	}
	if err = os.Mkdir(backup.Dir, 0o755); err != nil {
		err = fmt.Errorf("failed to create backup directory '%s': %w", backup.Dir, err)
		return
	}
	for _, f := range saveFiles {
		if err = copyFile(f, filepath.Join(backup.Dir, filepath.Base(f))); err != nil {
			err = fmt.Errorf("failed to copy save file '%s' to backup directory '%s': %w", f, backup.Dir, err)
			return
		}
	}
	if metadataErr := writeBackupMetadata(metadata, backup.Dir); metadataErr != nil {
		log.Warnf("failed to write backup metadata: %s", metadataErr)
	}
	return
}

func writeBackupMetadata(metadata BackupMetadata, dir string) error {
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode backup metadata: %w", err)
	}
	file := filepath.Join(dir, _metadataFilename)
	if err := os.WriteFile(file, encoded, 0o644); err != nil {
		return fmt.Errorf("failed to write backup metadata to '%s': %w", file, err)
	}
	return nil
}

func getBackups() (backups []Backup, err error) {
	dirs, _ := filepath.Glob(filepath.Join(_backupsDirectory, "Bak.*"))
	for _, dir := range dirs {
		backup, err := readBackup(dir)
		if err != nil {
			log.Warn(err)
			continue
		}
		backups = append(backups, backup)
	}
	slices.SortFunc(backups, func(b1, b2 Backup) int {
		c := b2.Metadata.CreatedAt.Compare(b1.Metadata.CreatedAt)
		if c != 0 {
			return c
		}
		return strings.Compare(b2.Dir, b1.Dir)
	})
	return
}

func readBackup(dir string) (backup Backup, err error) {
	backup.Dir = dir
	stat, err := os.Stat(dir)
	if err != nil {
		err = fmt.Errorf("failed to stat backup directory '%s': %w", dir, err)
		return
	}
	// Make sure there's at least one save file in the backup directory.
	saves, _ := filepath.Glob(filepath.Join(dir, "*.save"))
	if len(saves) == 0 {
		err = fmt.Errorf("failed to find .save files in backup directory '%s'", dir)
		return
	}
	// Load metadata.
	metadataFile := filepath.Join(dir, _metadataFilename)
	encoded, err := os.ReadFile(metadataFile)
	if err != nil {
		log.Warnf("failed to read backup metadata from '%s': %s", metadataFile, err)
	} else {
		if err = json.Unmarshal(encoded, &backup.Metadata); err != nil {
			log.Warnf("failed to decode backup metadata from '%s': %s", metadataFile, err)
		}
	}
	if !backup.Metadata.CreatedAt.IsZero() {
		// Successfully loaded metadata.
		return
	}
	// Fallback to parsing backup directory name.
	dirname := filepath.Base(dir)
	if dirname == _overwrittenBackupDirname {
		backup.Metadata.IsOverwritten = true
	} else {
		backup.Metadata.CreatedAt, err = time.Parse(_backupDirnameFormat, dirname)
		if err != nil {
			log.Warnf("unrecognized backup directory name '%s'", dirname)
		}
	}
	// Fallback to using the directory's modification time.
	if backup.Metadata.CreatedAt.IsZero() {
		backup.Metadata.CreatedAt = stat.ModTime()
	}
	return
}

func restoreBackup(backup Backup) (autoBackup Backup, err error) {
	log.Infof("restoring backup '%s'", backup.Dir)

	pattern := filepath.Join(backup.Dir, "*.save")
	saveFiles, _ := filepath.Glob(pattern)
	if len(saveFiles) == 0 {
		err = fmt.Errorf("failed to find save files '%s'", pattern)
		return
	}
	for _, expected := range _expectedSaveFiles {
		found := false
		for _, f := range saveFiles {
			if filepath.Base(f) == expected {
				found = true
				break
			}
		}
		if !found {
			log.Warnf("expected save file '%s' not found in backup directory '%s'", expected, backup.Dir)
		}
	}

	// Compare WorldSave.save to determine if game restart is required.
	var currentWorldSaveContent, backupWorldSaveContent []byte
	currentWorldSave := filepath.Join(_savesDirectory, "WorldSave.save")
	backupWorldSave := filepath.Join(backup.Dir, "WorldSave.save")
	currentWorldSaveContent, err = os.ReadFile(currentWorldSave)
	if err != nil {
		log.Warnf("failed to read %s: %s", currentWorldSave, err)
	}
	backupWorldSaveContent, err = os.ReadFile(backupWorldSave)
	if err != nil {
		log.Warnf("failed to read %s: %s", backupWorldSave, err)
	}
	gameRestartRequied := !bytes.Equal(currentWorldSaveContent, backupWorldSaveContent)

	// Refuse restore if game restart is required but game is running.
	if gameRestartRequied {
		var gameRunning bool
		gameRunning, err = processIsRunning(_againstTheStormExecutable)
		if err != nil {
			log.Warnf("failed to check if game is running, assuming it's not: %s", err)
		}
		if gameRunning {
			err = _errGameIsRunningRestoreRefused
			return
		}
	}

	// Create auto backup of current state.
	log.Info("creating auto backup of current state before overwriting")
	autoBackup, err = createBackup(BackupMetadata{
		IsOverwritten: true,
	})
	if err != nil {
		err = fmt.Errorf("failed to create auto backup of current state, refusing to overwrite: %w", err)
		return
	}
	log.Infof("created auto backup '%s' of current state", autoBackup.Dir)

	// Copy files over.
	for _, f := range saveFiles {
		dst := filepath.Join(_savesDirectory, filepath.Base(f))
		if err = copyFile(f, dst); err != nil {
			err = fmt.Errorf("failed to copy save file '%s' to save directory '%s': %w", f, _savesDirectory, err)
			return
		}
	}

	log.Infof("restored backup '%s'", backup.Dir)
	return
}

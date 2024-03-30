package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/winlabs/gowin32"

	"github.com/fanaticscripter/AtSS/log"
)

var (
	_eremiteGamesRootDirectory string
	_savesDirectory            string
)

var _expectedSaveFiles = []string{
	"MetaSave.save",
	"Profiles.save",
	"Save.save",
	"WorldSave.save",
}

func init() {
	// C:\Users\<username>\AppData\LocalLow\Eremite Games\Against the Storm
	localLowAppDataPath, err := gowin32.GetKnownFolderPath(gowin32.KnownFolderLocalAppDataLow)
	if err != nil {
		log.Fatalf("failed to get LocalAppDataLow folder path: %s", err)
	}
	_eremiteGamesRootDirectory = filepath.Join(localLowAppDataPath, "Eremite Games")
	_savesDirectory = filepath.Join(_eremiteGamesRootDirectory, "Against the Storm")
	if _, err := os.Stat(_savesDirectory); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("save directory '%s' does not exist", _savesDirectory)
		}
		log.Fatalf("error checking save directory '%s': %s", _savesDirectory, err)
	}

	// Backups are placed in a separate directory so that they aren't synced to Steam Cloud.
	_backupsDirectory = filepath.Join(_eremiteGamesRootDirectory, _backupRootDirname)
	if err := os.MkdirAll(_backupsDirectory, 0o755); err != nil {
		log.Fatalf("failed to create backups directory '%s': %s", _backupsDirectory, err)
	}
}

func getSaveAge() (lastModified time.Time, age time.Duration, err error) {
	pattern := filepath.Join(_savesDirectory, "*.save")
	saveFiles, _ := filepath.Glob(pattern)
	if len(saveFiles) == 0 {
		err = fmt.Errorf("failed to find save files '%s'", pattern)
		return
	}
	for _, f := range saveFiles {
		var stat fs.FileInfo
		stat, err = os.Stat(f)
		if err != nil {
			err = fmt.Errorf("failed to stat save file '%s': %w", f, err)
			return
		}
		if stat.ModTime().After(lastModified) {
			lastModified = stat.ModTime()
		}
	}
	if lastModified.After(time.Now()) {
		err = fmt.Errorf("last modified time of save files is in the future, probably wrong: %s", lastModified)
		return
	}
	age = time.Since(lastModified)
	return
}

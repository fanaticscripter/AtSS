package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/winlabs/gowin32"
	"golang.org/x/crypto/blake2b"

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

var _validate = validator.New(validator.WithRequiredStructEnabled())

type CompositeSave struct {
	MetaSave RawMetaSave
	Save     RawSave
}

// RawMetaSave is for MetaSave.save.
type RawMetaSave struct {
	Gameplay *struct {
		HasActiveGame *bool `json:"hasActiveGame" validate:"required"`
	} `json:"gameplay" validate:"required"`
}

// RawSave is for Save.save.
type RawSave struct {
	Gameplay *struct {
		Year   *int `json:"year" validate:"required"`   // 1-based
		Season *int `json:"season" validate:"required"` // 0, 1, 2
	} `json:"gameplay" validate:"required"`
}

// 0 for world map (no active settlement), 3n-2 for year n drizzle, 3n-1 for
// year n clearance, 3n for year n storm. -1 for invalid.
type SeasonId int

const _invalidSeasonId SeasonId = -1

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

func getSaveAge(dir string) (lastModified time.Time, age time.Duration, err error) {
	pattern := filepath.Join(dir, "*.save")
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

func hashSave(dir string) (hash string, err error) {
	pattern := filepath.Join(dir, "*.save")
	saveFiles, _ := filepath.Glob(pattern)
	if len(saveFiles) == 0 {
		err = fmt.Errorf("failed to find save files '%s'", pattern)
		return
	}
	h, _ := blake2b.New512(nil)
	for _, f := range saveFiles {
		var content []byte
		content, err = os.ReadFile(f)
		if err != nil {
			err = fmt.Errorf("failed to read save file '%s': %w", f, err)
			return
		}
		_, _ = h.Write(content)
		_, _ = h.Write([]byte("\000"))
	}
	hash = fmt.Sprintf("%x", h.Sum(nil))
	return
}

func readSave(dir string) (save CompositeSave, err error) {
	var content []byte

	metasavePath := filepath.Join(dir, "MetaSave.save")
	content, err = os.ReadFile(metasavePath)
	if err != nil {
		err = fmt.Errorf("failed to read '%s': %w", metasavePath, err)
		return
	}
	err = json.Unmarshal(content, &save.MetaSave)
	if err != nil {
		err = fmt.Errorf("failed to parse '%s': %w", metasavePath, err)
		return
	}
	err = _validate.Struct(save.MetaSave)
	if err != nil {
		err = fmt.Errorf("failed to validate parsed '%s': %w", metasavePath, err)
		return
	}

	savePath := filepath.Join(dir, "Save.save")
	content, err = os.ReadFile(savePath)
	if err != nil {
		err = fmt.Errorf("failed to read '%s': %w", savePath, err)
		return
	}
	err = json.Unmarshal(content, &save.Save)
	if err != nil {
		err = fmt.Errorf("failed to parse '%s': %w", savePath, err)
		return
	}
	err = _validate.Struct(save.Save)
	if err != nil {
		err = fmt.Errorf("failed to validate parsed '%s': %w", savePath, err)
	}
	return
}

func (s CompositeSave) SeasonId() (sid SeasonId) {
	defer func() {
		if r := recover(); r != nil {
			sid = _invalidSeasonId
		}
	}()
	if !*s.MetaSave.Gameplay.HasActiveGame {
		return 0
	}
	return SeasonId(*s.Save.Gameplay.Year*3 - 2 + *s.Save.Gameplay.Season)
}

func (sid SeasonId) String() string {
	if sid < 0 {
		return "invalid"
	}
	if sid == 0 {
		return "world map"
	}
	year := (int(sid) + 2) / 3
	var seasonName string
	switch sid % 3 {
	case 1:
		seasonName = "drizzle"
	case 2:
		seasonName = "clearance"
	case 0:
		seasonName = "storm"
	}
	return fmt.Sprintf("Y%d %s", year, seasonName)
}

func (sid SeasonId) IsValid() bool {
	return sid >= 0
}

func (sid SeasonId) IsWorldMap() bool {
	return sid == 0
}

func (sid SeasonId) IsDrizzle() bool {
	return sid%3 == 1
}

func (sid SeasonId) IsClearance() bool {
	return sid%3 == 2
}

func (sid SeasonId) IsStorm() bool {
	return sid%3 == 0
}

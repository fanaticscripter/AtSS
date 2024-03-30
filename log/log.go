package log

import (
	"fmt"
	"os"

	"github.com/inconshreveable/mousetrap"
	"github.com/sirupsen/logrus"
)

var log = &logrus.Logger{
	Out:       os.Stderr,
	Formatter: &logrus.TextFormatter{ForceColors: true},
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.InfoLevel,
}

// Exit is a wrapper around os.Exit that waits for the user to press Enter
// before exiting if the program was started by Explorer.
func Exit(code int) {
	if mousetrap.StartedByExplorer() {
		fmt.Println()
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
	os.Exit(code)
}

func Fatal(v ...any) {
	log.Log(logrus.FatalLevel, v...)
	Exit(1)
}

func Fatalf(format string, v ...any) {
	Fatal(fmt.Sprintf(format, v...))
}

func Error(v ...any) {
	log.Error(v...)
}

func Errorf(format string, v ...any) {
	log.Errorf(format, v...)
}

func Warn(v ...any) {
	log.Warn(v...)
}

func Warnf(format string, v ...any) {
	log.Warnf(format, v...)
}

func Info(v ...any) {
	log.Info(v...)
}

func Infof(format string, v ...any) {
	log.Infof(format, v...)
}

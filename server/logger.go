package server

import (
	"log"
	"os"
)

// LogLevel type and logger
const (
	DEBUG = iota
	INFO
	WARN
	ERROR
)

var logLevel = INFO

func SetLogLevelFromEnv() {
	lvl := os.Getenv("NGOPEN_LOG_LEVEL")
	switch lvl {
	case "DEBUG":
		logLevel = DEBUG
	case "INFO":
		logLevel = INFO
	case "WARN":
		logLevel = WARN
	case "ERROR":
		logLevel = ERROR
	}
}

func LogDebug(format string, v ...interface{}) {
	if logLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}
func LogInfo(format string, v ...interface{}) {
	if logLevel <= INFO {
		log.Printf("[INFO] "+format, v...)
	}
}
func LogWarn(format string, v ...interface{}) {
	if logLevel <= WARN {
		log.Printf("[WARN] "+format, v...)
	}
}
func LogError(format string, v ...interface{}) {
	if logLevel <= ERROR {
		log.Printf("[ERROR] "+format, v...)
	}
}

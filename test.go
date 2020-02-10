package main

import (
	"os"
	"path/filepath"
)

func main() {
	logPath, _ := os.Getwd()
	logPath = filepath.Join(logPath, "wzbtc-log")
	initLogRotator(logPath)
	setLogLevels("trace")

	btcdLog.Tracef("test netlog, %d", 11)
	btcdLog.Debugf("test netlog, %d", 11)
	btcdLog.Infof("test netlog, %d", 11)
	btcdLog.Warnf("test netlog, %d", 11)
	btcdLog.Errorf("test netlog, %d", 11)
	btcdLog.Fatalf("test netlog, %d", 11)
}

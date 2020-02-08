package main

import (
	"fmt"
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/peer"
	"os"
	"path/filepath"
)

type logWriter struct{}

var (
	backendLog = bclog.NewBackend(logWriter{})
	logRotator *bclog.Rotator
	netLog     = backendLog.Logger("NET")
)

func (log logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	logRotator.Write(p)
	return len(p), nil
}

func init() {
	peer.UseLogger(netLog)
}

var subsystemLoggers = map[string]bclog.Logger{
	"NET": netLog,
}

func initLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory:%v\n", err)
		os.Exit(1)
	}
	r, err := bclog.New(logFile, 10*1024, false, 3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	logRotator = r
}

func setLogLevel(subsystemID string, logLevel string) {
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	level, _ := bclog.LevelFromStrings(logLevel)
	logger.SetLevel(level)
}

func setLogLevels(logLevel string) {
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

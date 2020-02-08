package main

import (
	"github.com/wangzhen0101/wzbtc/peer"
	"os"
	"path/filepath"
)

func main() {
	logPath, _ := os.Getwd()
	logPath = filepath.Join(logPath, "wzbtc-log")
	initLogRotator(logPath)
	setLogLevels("trace")

	netLog.Tracef("test netlog, %d", 11)
	netLog.Debugf("test netlog, %d", 11)
	netLog.Infof("test netlog, %d", 11)
	netLog.Warnf("test netlog, %d", 11)
	netLog.Errorf("test netlog, %d", 11)
	netLog.Fatalf("test netlog, %d", 11)
	peer.TestNetLog("call by main")
}

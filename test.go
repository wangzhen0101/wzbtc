package main

import (
	"github.com/wangzhen0101/wzbtc/peer"
	"time"
)

func main() {
	initLogRotator("F:\\goproject\\src\\github.com\\wangzhen0101\\wzbtc\\wzbtc-log")
	setLogLevels("trace")
	for {
		netLog.Tracef("test netlog, %d", 11)
		netLog.Debugf("test netlog, %d", 11)
		netLog.Infof("test netlog, %d", 11)
		netLog.Warnf("test netlog, %d", 11)
		netLog.Errorf("test netlog, %d", 11)
		netLog.Fatalf("test netlog, %d", 11)
		peer.TestNetLog("call by main")
		time.Sleep(1000)
	}

}

package main

import (
	"os"
	"time"
)

var (
	cfg *config
)

func btcdMain(serverChan chan<- *server) error {
	tcfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = tcfg
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	DumpCfg(cfg)

	interrupt := interruptListener()
	defer btcdLog.Info("Shutdown complete")

	btcdLog.Infof("Version %s", version())

	btcdLog.Info("start success.")

	time.Sleep(time.Second * 3)

	if interruptRequested(interrupt) {
		return nil
	}

	return nil
}

func main() {

	if err := btcdMain(nil); err != nil {
		os.Exit(1)
	}
}

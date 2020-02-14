package main

import (
	"os"
	"os/signal"
)

var shutdownRequestChannel = make(chan struct{})

var interruptSignals = []os.Signal{os.Interrupt}

func interruptListener() <-chan struct{} {
	c := make(chan struct{})
	go func() {
		interruptChannel := make(chan os.Signal, 1)
		signal.Notify(interruptChannel, interruptSignals...)

		select {
		case sig := <-interruptChannel:
			log.Infof("Received signal (%s).  Shutting down...", sig)
			//if db != nil {
			//	log.Infof("Close db...")
			//	db.Close()
			//	db = nil
			//}
			//os.Exit(0)

		case <-shutdownRequestChannel:
			log.Info("Shutdown requested.  Shutting down...")

		}
		close(c)
		for {
			select {
			case sig := <-interruptChannel:
				log.Infof("Received signal (%s).  Already "+
					"shutting down...", sig)

			case <-shutdownRequestChannel:
				log.Info("Shutdown requested.  Already " +
					"shutting down...")
			}
		}

	}()

	return c
}

func interruptRequested(interrupted <-chan struct{}) bool {
	select {
	case <-interrupted:
		return true
	default:
	}

	return false
}

package main

import (
	"fmt"
	"github.com/wangzhen0101/wzbtc/database"
	"time"
)

var (
	cfgGetKey = &cmdGetKey{Idx: -1, Key: "", Loop: false}
)

type cmdGetKey struct {
	Key         string `short:"k" long:"key" description:"the index of the block"`
	Loop        bool   `short:"l" long:"loop" description:"loop or not"`
	Idx         int32  `short:"i" long:"idx" description:"idx for key"`
	RawKey      bool   `short:"r" long:"rawkey" description:"raw key for key"`
	FetchAllKey bool   `short:"f" long:"fetchallkey" description:"raw key for key"`
}

func getKeyByName(key string) []byte {
	buf := make([]byte, 1024)
	err := db.View(func(tx database.Tx) error {
		// Store a key/value pair directly in the metadata bucket.
		// Typically a nested bucket would be used for a given feature,
		// but this example is using the metadata bucket directly for
		// simplicity.
		//key := []byte(fmt.Sprintf("mykey%06d", idx))
		key := []byte(key)
		copy(buf, tx.Metadata().Get(key))
		return nil
	})
	if err != nil {
		log.Error(err)
		return nil
	}
	return buf
}

func getKeyByIdx(idx int32) []byte {
	var buf = make([]byte, 16)
	err := db.View(func(tx database.Tx) error {
		// Store a key/value pair directly in the metadata bucket.
		// Typically a nested bucket would be used for a given feature,
		// but this example is using the metadata bucket directly for
		// simplicity.
		key := []byte(fmt.Sprintf("mykey%06d", idx))
		copy(buf, tx.Metadata().Get(key))
		return nil
	})
	if err != nil {
		log.Error(err)
		return nil
	}
	return buf
}

func (cmd *cmdGetKey) Execute(args []string) error {
	log.Infof("exec cmd cfgGetKey:index-[%s], loop-[%t]", cfgGetKey.Key, cfgGetKey.Loop)

	for {
		if cfgGetKey.Idx > -1 {
			log.Infof("get key idx:%d, value:%s", cfgGetKey.Idx, getKeyByIdx(cfgGetKey.Idx))
		} else {
			log.Infof("get key :%s, value:%s", cfgGetKey.Key, getKeyByName(cfgGetKey.Key))
		}

		if !cfgGetKey.Loop {
			break
		} else {
			//idx++
		}

		if interruptRequested(interrupt) {
			return nil
		}

		time.Sleep(time.Millisecond * 100)
	}

	return nil
}

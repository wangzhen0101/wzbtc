package main

import (
	"fmt"
	"github.com/wangzhen0101/wzbtc/database"
	"time"
)

var (
	cfgGetBlock = &cmdGetBlock{Idx: -1, Loop: false}
)

type cmdGetBlock struct {
	Idx  int64 `short:"i" long:"index" description:"the index of the block"`
	Loop bool  `short:"l" long:"loop" description:"loop or not"`
}

func getBlock(idx uint32) []byte {
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

func (cmd *cmdGetBlock) Execute(args []string) error {
	log.Infof("exec cmd getblock:index-[%d], loop-[%t]", cfgGetBlock.Idx, cfgGetBlock.Loop)

	openTestDb()

	idx := uint32(cfgGetBlock.Idx)

	for {

		Block100000.Header.Nonce = idx

		log.Infof("get key idx:%d, value:%s", idx, getBlock(idx))

		if !cfgGetBlock.Loop {
			break
		} else {
			idx++
		}

		if interruptRequested(interrupt) {
			return nil
		}

		time.Sleep(time.Millisecond * 100)
	}

	return nil

	return nil
}

package main

import (
	"github.com/wangzhen0101/wzbtc/database"
	"time"
)

var (
	cfgGetKey = &cmdGetKey{Key: "", Loop: false}
)

type cmdGetKey struct {
	Key  string `short:"k" long:"key" description:"the index of the block"`
	Loop bool   `short:"l" long:"loop" description:"loop or not"`
}

func getKey(key string) []byte {
	var buf = make([]byte, 16)
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

func (cmd *cmdGetKey) Execute(args []string) error {
	log.Infof("exec cmd cfgGetKey:index-[%s], loop-[%t]", cfgGetKey.Key, cfgGetKey.Loop)

	openTestDb()

	//idx := uint32(cfgGetKey.key)

	for {

		//Block100000.Header.Nonce = idx

		log.Infof("get key idx:%d, value:%s", cfgGetKey.Key, getKey(cfgGetKey.Key))

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

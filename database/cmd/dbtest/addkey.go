package main

import (
	"fmt"
	"github.com/wangzhen0101/wzbtc/database"
	"time"
)

var (
	cfgAddKey = &cmdAddKey{Idx: -1, Loop: false}
)

type cmdAddKey struct {
	Idx  int64 `short:"i" long:"index" description:"the index of the block"`
	Loop bool  `short:"l" long:"loop" description:"loop or not"`
}

func updateKey(idx uint32) error {
	err := db.Update(func(tx database.Tx) error {
		// Store a key/value pair directly in the metadata bucket.
		// Typically a nested bucket would be used for a given feature,
		// but this example is using the metadata bucket directly for
		// simplicity.
		key := []byte(fmt.Sprintf("mykey%06d", idx))
		value := []byte(fmt.Sprintf("myvalue%06d", idx))
		if err := tx.Metadata().Put(key, value); err != nil {
			return err
		}

		//// Read the key back and ensure it matches.
		//if !bytes.Equal(tx.Metadata().Get(key), value) {
		//	return fmt.Errorf("unexpected value for key '%s'", key)
		//}
		log.Infof("set Key:%s value:%s", key, tx.Metadata().Get(key))

		//// Create a new nested bucket under the metadata bucket.
		//nestedBucketKey := []byte("mybucket")
		//nestedBucket, err := tx.Metadata().CreateBucket(nestedBucketKey)
		//if err != nil {
		//	return err
		//}
		//
		//// The key from above that was set in the metadata bucket does
		//// not exist in this new nested bucket.
		//if nestedBucket.Get(key) != nil {
		//	log.Errorf("key '%s' is not expected nil", key)
		//	return errors.New("key is not exist")
		//}

		return nil
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (cmd *cmdAddKey) Execute(args []string) error {
	log.Infof("exec cmd cfgAddKey:index-[%d], loop-[%t]", cfgAddKey.Idx, cfgAddKey.Loop)

	openTestDb()

	idx := uint32(cfgAddKey.Idx)

	for {

		Block100000.Header.Nonce = idx

		_ = updateKey(idx)

		log.Infof("update key idx:%d", idx)

		if !cfgAddKey.Loop {
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
}

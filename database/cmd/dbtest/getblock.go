package main

import (
	"github.com/wangzhen0101/btcutil"
	"github.com/wangzhen0101/wzbtc/chaincfg/chainhash"
	"github.com/wangzhen0101/wzbtc/database"
)

var (
	cfgGetBlock = &cmdGetBlock{Key: "", Loop: false}
)

type cmdGetBlock struct {
	Key  string `short:"k" long:"key" description:"the key of the block"`
	Loop bool   `short:"l" long:"loop" description:"loop or not"`
}

func printBlock(block *btcutil.Block) {
	log.Infof("block.MsgBlock().Header.Version    :%d", block.MsgBlock().Header.Version)
	log.Infof("block.MsgBlock().Header.PrevBlock  :%s", block.MsgBlock().Header.PrevBlock)
	log.Infof("block.MsgBlock().Header.BlockHash  :%s", block.MsgBlock().Header.BlockHash())
	log.Infof("block.MsgBlock().Header.MerkleRoot :%s", block.MsgBlock().Header.MerkleRoot)
	log.Infof("block.MsgBlock().Header.Bits       :%d", block.MsgBlock().Header.Bits)
	log.Infof("block.MsgBlock().Header.Nonce      :%d", block.MsgBlock().Header.Nonce)
}

func getBlock(key string) *btcutil.Block {
	buf := make([]byte, 1024)
	err := db.View(func(tx database.Tx) error {
		// Store a key/value pair directly in the metadata bucket.
		// Typically a nested bucket would be used for a given feature,
		// but this example is using the metadata bucket directly for
		// simplicity.
		//key := []byte(fmt.Sprintf("mykey%06d", idx))
		//key := chainhash.Hash(key)
		hashKey, err := chainhash.NewHashFromStr(key)
		if err != nil {
			log.Errorf("can not convert to hash :%s", key)
			return err
		}

		buf, err = tx.FetchBlock(hashKey)
		if err != nil {
			log.Errorf("can not fetch Block:%s", hashKey)
			return err
		}

		return nil
	})
	if err != nil {
		log.Error(err)
		return nil
	}

	block, err := btcutil.NewBlockFromBytes(buf)
	if err != nil {
		log.Errorf("NewBlockFromBytes fail:%s", err)
		return nil
	}

	printBlock(block)

	return block
}

func (cmd *cmdGetBlock) Execute(args []string) error {
	log.Infof("exec cmd getblock:index-[%s], loop-[%t]", cfgGetBlock.Key, cfgGetBlock.Loop)

	openTestDb()

	for {

		getBlock(cfgGetBlock.Key)

		log.Infof("get key:%s success.", cfgGetBlock.Key)

		if !cfgGetBlock.Loop {
			break
		}

		if interruptRequested(interrupt) {
			return nil
		}

		//time.Sleep(time.Millisecond * 100)
	}

	return nil
}

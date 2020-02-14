package main

import (
	"github.com/wangzhen0101/btcutil"
	"github.com/wangzhen0101/wzbtc/database"
)

var (
	cfgAddBlock = &cmdAddBlock{Idx: -1, Loop: false}
)

type cmdAddBlock struct {
	Idx  int64 `short:"i" long:"index" description:"the index of the block"`
	Loop bool  `short:"l" long:"loop" description:"loop or not"`
}

func updateBlock(idx int32) error {
	block := btcutil.NewBlock(&Block100000)
	block.MsgBlock().Header.Nonce = uint32(idx)
	block.SetHeight(idx)
	blockBytes, err := block.Bytes()
	if err != nil {
		log.Errorf("block to bytes fail:%s", err)
		return err
	}

	err = db.Update(func(tx database.Tx) error {

		//db.Update会自动提交
		tx.StoreBlock(block)

		log.Infof("commit %06d block hash:%s, block len:%d", idx, block.Hash(), len(blockBytes))

		return nil
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (cmd *cmdAddBlock) Execute(args []string) error {
	log.Infof("exec cmd addblock:index-[%d], loop-[%t]", cfgAddBlock.Idx, cfgAddBlock.Loop)

	openTestDb()

	idx := int32(cfgAddBlock.Idx)

	for {

		_ = updateBlock(idx)

		//log.Infof("update key idx:%d", idx)

		if !cfgAddBlock.Loop {
			break
		} else {
			idx++
		}

		if interruptRequested(interrupt) {
			return nil
		}

		//time.Sleep(time.Millisecond * 100)
	}

	return nil
}

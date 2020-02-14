package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/wangzhen0101/wzbtc/database"
	"hash/crc32"
)

var (
	cfgInfo         = &infoCfg{}
	byteOrder       = binary.LittleEndian
	writeLocKeyName = []byte("ffldb-writeloc")
	castagnoli      = crc32.MakeTable(crc32.Castagnoli)
)

type infoCfg struct {
}

// deserializeWriteRow deserializes the write cursor location stored in the
// metadata.  Returns ErrCorruption if the checksum of the entry doesn't match.
func deserializeWriteRow(writeRow []byte) (uint32, uint32, error) {
	// Ensure the checksum matches.  The checksum is at the end.
	gotChecksum := crc32.Checksum(writeRow[:8], castagnoli)
	wantChecksumBytes := writeRow[8:12]
	wantChecksum := byteOrder.Uint32(wantChecksumBytes)
	if gotChecksum != wantChecksum {
		str := fmt.Sprintf("metadata for write cursor does not match "+
			"the expected checksum - got %d, want %d", gotChecksum,
			wantChecksum)
		return 0, 0, errors.New(str)
	}

	fileNum := byteOrder.Uint32(writeRow[0:4])
	fileOffset := byteOrder.Uint32(writeRow[4:8])
	return fileNum, fileOffset, nil
}

func (cmd *infoCfg) Execute(args []string) error {
	log.Infof("database info:%s", database.SupportedDrivers())

	openTestDb()

	var curFileNum, curOffset uint32

	err := db.View(func(tx database.Tx) error {
		writeRow := tx.Metadata().Get(writeLocKeyName)
		if writeRow == nil {
			str := "write cursor does not exist"
			return errors.New(str)
		}

		var err error
		curFileNum, curOffset, err = deserializeWriteRow(writeRow)
		return err
	})

	if err != nil {
		log.Error(err)
		return err
	}

	log.Infof("curFileNum is :%d, curOffset is :%d", curFileNum, curOffset)

	return nil
}

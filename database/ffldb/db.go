package ffldb

import (
	"github.com/wangzhen0101/wzbtc/database"
	"github.com/wangzhen0101/wzbtc/wire"
)

func openDB(dbPath string, network wire.BitcoinNet, create bool) (database.DB, error) {
	// Error if the database doesn't exist and the create flag is not set.

	log.Fatalf("OpenDb is empyt now.")
	return nil, nil
}

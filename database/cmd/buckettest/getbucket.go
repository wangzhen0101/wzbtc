package main

import (
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/database"
	"github.com/wangzhen0101/wzbtc/wire"
	"os"
	"path/filepath"
)

const (
	blockDbNamePrefix = "blocks"
	DbType            = "ffldb"
)

var (
	log       bclog.Logger
	db        database.DB
	interrupt <-chan struct{}
)

func blockDbPath(dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(cfg.DataDir, dbName)
	return dbPath
}

func openTestDb() {
	//open the database
	dbPath := blockDbPath(DbType)
	tdb, err := database.Open(DbType, dbPath, wire.MainNet)
	if err != nil {
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode != database.ErrDbDoesNotExist {
			log.Errorf("open fail :%s", err)
			return
		}

		err = os.MkdirAll(cfg.DataDir, 0700)
		if err != nil {
			log.Errorf("MkdirAll fail :%s", err)
			return
		}

		tdb, err = database.Create(DbType, dbPath, wire.MainNet)
		if err != nil {
			log.Errorf("Create fail :%s", err)
			return
		}
	}

	log.Infof("open db path:%s success.", dbPath)
	db = tdb
}

func closeDb() {
	if db != nil {
		log.Infof("close db...")
		db.Close()
	}
}

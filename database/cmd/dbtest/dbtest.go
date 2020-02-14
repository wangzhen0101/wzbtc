package main

import (
	"github.com/jessevdk/go-flags"
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/database"
	_ "github.com/wangzhen0101/wzbtc/database/ffldb"
	"github.com/wangzhen0101/wzbtc/wire"
	"os"
	"path/filepath"
	"strings"
)

const (
	blockDbNamePrefix = "blocks"
	DbType            = "ffldb"
)

var (
	log       bclog.Logger
	cfg       = &cmdCfg{}
	db        database.DB
	interrupt <-chan struct{}
)

type cmdCfg struct {
	DataDir string `short:"d" long:"datadir" description:"data dir"`
}

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

func main() {
	backendLogger := bclog.NewBackend(os.Stdout)
	defer os.Stdout.Sync()
	log = backendLogger.Logger("MAIN")
	dbLog := backendLogger.Logger("BCDB")
	dbLog.SetLevel(bclog.LevelDebug)
	log.SetLevel(bclog.LevelDebug)
	database.UseLogger(dbLog)

	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	//log.Debug(appName)
	parserFlags := flags.Options(flags.HelpFlag | flags.PassDoubleDash)
	parser := flags.NewNamedParser(appName, parserFlags)
	parser.AddGroup("Global Options", "", cfg)
	parser.AddCommand("addblock",
		"add block to the storage system",
		"add block the the storage system by index.", cfgAddBlock)
	parser.AddCommand("getblock",
		"get block to the storage system",
		"get block the the storage system by index.", cfgGetBlock)

	parser.AddCommand("addkey",
		"add key to the database",
		"add key the the database by index.", cfgAddKey)
	parser.AddCommand("getkey",
		"get key to the database",
		"get key the the database by index.", cfgGetKey)

	parser.AddCommand("info",
		"print the database info",
		"print the database info by name.", cfgInfo)

	defer closeDb()

	interrupt = interruptListener()
	defer log.Info("In main Shutdown complete")

	// Parse command line and invoke the Execute function for the specified
	// command.
	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		} else {
			log.Error(err)
		}

		return
	}

	if interruptRequested(interrupt) {
		return
	}

	return
}

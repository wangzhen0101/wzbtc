package main

import (
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/blockchain"
	"github.com/wangzhen0101/wzbtc/blockchain/indexers"
	"github.com/wangzhen0101/wzbtc/chaincfg"
	"github.com/wangzhen0101/wzbtc/database"
	_ "github.com/wangzhen0101/wzbtc/database/ffldb"
	"github.com/wangzhen0101/wzbtc/txscript"
	"os"
)

type logWriter struct{}

func (l logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return len(p), nil
}

type tmpServer struct {
	chain        *blockchain.BlockChain
	chainParams  *chaincfg.Params
	sigCache     *txscript.SigCache
	hashCache    *txscript.HashCache
	timeSource   blockchain.MedianTimeSource
	txIndex      *indexers.TxIndex
	addrIndex    *indexers.AddrIndex
	cfIndex      *indexers.CfIndex
	indexManager blockchain.IndexManager
	db           database.DB
	checkpoints  []chaincfg.Checkpoint
	interrupt    <-chan struct{}
}

func newServer(db database.DB, interrupt <-chan struct{}) (*tmpServer, error) {
	s := &tmpServer{
		chainParams: activeNetParams,
		db:          db,
		sigCache:    txscript.NewSigCache(txscript.MaxScriptSize),
		hashCache:   txscript.NewHashCache(txscript.MaxScriptSize),
		timeSource:  blockchain.NewMedianTime(),
		checkpoints: activeNetParams.Checkpoints,
		interrupt:   interrupt,
	}

	var indexes []indexers.Indexer
	s.txIndex = indexers.NewTxIndex(db)
	indexes = append(indexes, s.txIndex)
	s.addrIndex = indexers.NewAddrIndex(db, activeNetParams)
	indexes = append(indexes, s.addrIndex)
	s.cfIndex = indexers.NewCfIndex(db, activeNetParams)
	indexes = append(indexes, s.cfIndex)
	s.indexManager = indexers.NewManager(db, indexes)

	return s, nil
}

var (
	backendLog      = bclog.NewBackend(logWriter{})
	log             = backendLog.Logger("TEST")
	chanLog         = backendLog.Logger("CHAN")
	dataLog         = backendLog.Logger("DATA")
	activeNetParams = &chaincfg.MainNetParams
	mainchain       *blockchain.BlockChain
)

func init() {
	log.SetLevel(bclog.LevelTrace)
	chanLog.SetLevel(bclog.LevelTrace)
	dataLog.SetLevel(bclog.LevelTrace)
	blockchain.UseLogger(chanLog)
	database.UseLogger(dataLog)

}

func openDb() (database.DB, error) {
	dbPath := "F:\\goproject\\data\\mainnet\\blocks_ffldb"

	log.Infof("Loading block database from '%s'", dbPath)
	db, err := database.Open("ffldb", dbPath, activeNetParams.Net)
	if err != nil {
		log.Infof("Loading block database from '%s'", err)
		return nil, err
	}

	log.Info("Block database loaded")
	return db, nil
}

func printBucket(db database.DB, bucket database.Bucket) {
	err := db.View(func(tx database.Tx) error {
		if bucket == nil {
			bucket = tx.Metadata()
		}
		//c := mataBucket.Cursor()
		//for ok:=c.First(); ok; ok = c.Next() {
		//	//log.Infof("key:%s, value:%s", c.Key(),c.Value())
		//	log.Infof("key:%s", c.Key())
		//}

		bucket.ForEach(func(k, v []byte) error {
			log.Infof("key:%s, value:%x", k, v)
			return nil
		})

		bucket.ForEachBucket(func(k []byte) error {
			log.Infof("bucket-name:%s", k)
			childBucket := bucket.Bucket(k)
			printBucket(db, childBucket)
			return nil
		})

		return nil
	})

	if err != nil {
		return
	}
}

func main() {
	// Load the block database.
	db, err := openDb()
	if err != nil {
		log.Errorf("%v", err)
		return
	}
	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		log.Infof("Gracefully shutting down the database...")
		db.Close()
	}()

	//printBucket(db, nil)

	s, err := newServer(db, nil)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	s.chain, err = blockchain.New(&blockchain.Config{
		DB:           s.db,
		Interrupt:    s.interrupt,
		ChainParams:  s.chainParams,
		Checkpoints:  s.checkpoints,
		TimeSource:   s.timeSource,
		SigCache:     s.sigCache,
		IndexManager: s.indexManager,
		HashCache:    s.hashCache,
	})

	if err != nil {
		log.Errorf("%v", err)
		return
	}

}

package main

import (
	"github.com/wangzhen0101/wzbtc/addrmgr"
	"github.com/wangzhen0101/wzbtc/bclog"
	"github.com/wangzhen0101/wzbtc/blockchain"
	"github.com/wangzhen0101/wzbtc/chaincfg"
	"github.com/wangzhen0101/wzbtc/connmgr"
	"github.com/wangzhen0101/wzbtc/netsync"
	"os"
	"path/filepath"
)

type logWriter struct{}

func (l logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return len(p), nil
}

var (
	backendLog      = bclog.NewBackend(logWriter{})
	log             = backendLog.Logger("TEST")
	amgrLog         = backendLog.Logger("ADMR")
	cngrLog         = backendLog.Logger("CNGR")
	syncLog         = backendLog.Logger("SYNC")
	activeNetParams = &chaincfg.MainNetParams
	addrManager     *addrmgr.AddrManager
	connManager     *connmgr.ConnManager
	syncManager     *netsync.SyncManager
	chain           *blockchain.BlockChain
)

func init() {
	log.SetLevel(bclog.LevelTrace)
	amgrLog.SetLevel(bclog.LevelTrace)
	cngrLog.SetLevel(bclog.LevelTrace)
	syncLog.SetLevel(bclog.LevelTrace)
	connmgr.UseLogger(amgrLog)
	addrmgr.UseLogger(cngrLog)
	netsync.UseLogger(syncLog)
}

func main() {
	cmdPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Infof("get path fial:%s", err)
	}
	log.Infof("cmdPath:%s", cmdPath)

	//s.syncManager, err = netsync.New(&netsync.Config{
	//	PeerNotifier:       &s,
	//	Chain:              s.chain,
	//	TxMemPool:          s.txMemPool,
	//	ChainParams:        s.chainParams,
	//	DisableCheckpoints: cfg.DisableCheckpoints,
	//	MaxPeers:           cfg.MaxPeers,
	//	FeeEstimator:       s.feeEstimator,
	//})
	//if err != nil {
	//	return nil, err
	//}
	chain, err = blockchain.New(&blockchain.Config{
		DB:           s.db,
		Interrupt:    interrupt,
		ChainParams:  s.chainParams,
		Checkpoints:  checkpoints,
		TimeSource:   s.timeSource,
		SigCache:     s.sigCache,
		IndexManager: indexManager,
		HashCache:    s.hashCache,
	})
	if err != nil {
		return nil, err
	}

	syncManager, err := netsync.New(&netsync.Config{
		PeerNotifier:       nil, //指向server自身
		Chain:              nil, //区块链
		TxMemPool:          nil,
		ChainParams:        activeNetParams,
		DisableCheckpoints: false,
		MaxPeers:           0,
		FeeEstimator:       nil,
	})
}

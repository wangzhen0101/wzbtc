package ffldb

import (
	"fmt"
	"github.com/wangzhen0101/btcutil"
	"github.com/wangzhen0101/wzbtc/chaincfg/chainhash"
	"github.com/wangzhen0101/wzbtc/database"
	"github.com/wangzhen0101/wzbtc/database/internal/treap"
	"sync"
)

// pendingBlock houses a block that will be written to disk when the database
// transaction is commi
//
//
//tted.
type pendingBlock struct {
	hash  *chainhash.Hash
	bytes []byte
}

type transaction struct {
	managed        bool             // Is the transaction managed?
	closed         bool             // Is the transaction closed?
	writable       bool             // Is the transaction writable?
	db             *db              // DB instance the tx was created from.
	snapshot       *dbCacheSnapshot // Underlying snapshot for txns.
	metaBucket     *bucket          // The root metadata bucket.
	blockIdxBucket *bucket          // The block index bucket.

	// Blocks that need to be stored on commit.  The pendingBlocks map is
	// kept to allow quick lookups of pending data by block hash.
	pendingBlocks    map[chainhash.Hash]int
	pendingBlockData []pendingBlock

	// Keys that need to be stored or deleted on commit.
	pendingKeys   *treap.Mutable
	pendingRemove *treap.Mutable

	// Active iterators that need to be notified when the pending keys have
	// been updated so the cursors can properly handle updates to the
	// transaction state.
	activeIterLock sync.RWMutex
	activeIters    []*treap.Iterator
}

var _ database.Tx = (*transaction)(nil)

func (tx *transaction) checkClosed() error {
	if tx.closed {
		return makeDbErr(database.ErrTxClosed, errTxClosedStr, nil)
	}
	return nil
}

// hasKey returns whether or not the provided key exists in the database while
// taking into account the current transaction state.
func (tx *transaction) hasKey(key []byte) bool {
	// When the transaction is writable, check the pending transaction
	// state first.
	if tx.writable {
		if tx.pendingRemove.Has(key) {
			return false
		}
		if tx.pendingKeys.Has(key) {
			return true
		}
	}

	// Consult the database cache and underlying database.
	return tx.snapshot.Has(key)
}

// hasBlock returns whether or not a block with the given hash exists.
func (tx *transaction) hasBlock(hash *chainhash.Hash) bool {
	// Return true if the block is pending to be written on commit since
	// it exists from the viewpoint of this transaction.
	if _, exists := tx.pendingBlocks[*hash]; exists {
		return true
	}

	return tx.hasKey(bucketizedKey(blockIdxBucketID, hash[:]))
}


func (tx *transaction) Metadata() database.Bucket {
	return tx.metaBucket
}
func (tx *transaction) StoreBlock(block *btcutil.Block) error {
	// Ensure transaction state is valid.
	if err := tx.checkClosed(); err != nil {
		return err
	}

	// Ensure the transaction is writable.
	if !tx.writable {
		str := "store block requires a writable database transaction"
		return makeDbErr(database.ErrTxNotWritable, str, nil)
	}

	// Reject the block if it already exists.
	blockHash := block.Hash()
	if tx.hasBlock(blockHash) {
		str := fmt.Sprintf("block %s already exists", blockHash)
		return makeDbErr(database.ErrBlockExists, str, nil)
	}

	blockBytes, err := block.Bytes()
	if err != nil {
		str := fmt.Sprintf("failed to get serialized bytes for block %s",
			blockHash)
		return makeDbErr(database.ErrDriverSpecific, str, err)
	}

	if tx.pendingBlocks == nil {
		tx.pendingBlocks = make(map[chainhash.Hash]int)
	}

	tx.pendingBlocks[*blockHash] = len(tx.pendingBlockData)
	tx.pendingBlockData = append(tx.pendingBlockData, pendingBlock{
		hash:  blockHash,
		bytes: blockBytes,
	})

	log.Tracef("Added block %s to pending blocks", blockHash)

	return nil
}


func (tx *transaction) HasBlock(hash *chainhash.Hash) (bool, error) {}
func (tx *transaction) HasBlocks(hashes []chainhash.Hash) ([]bool, error) {}
func (tx *transaction) FetchBlockHeader(hash *chainhash.Hash) ([]byte, error) {}
func (tx *transaction) FetchBlockHeaders(hashes []chainhash.Hash) ([][]byte, error){}
func (tx *transaction) FetchBlock(hash *chainhash.Hash) ([]byte, error){}
func (tx *transaction) FetchBlocks(hashes []chainhash.Hash) ([][]byte, error){}
func (tx *transaction) FetchBlockRegion(region *database.BlockRegion) ([]byte, error){}
func (tx *transaction) FetchBlockRegions(regions []database.BlockRegion) ([][]byte, error){}
func (tx *transaction) Commit() error{}
func (tx *transaction) Rollback() error{}
